package usecase

import (
	"context"
	"errors"
	"strings"

	"monorepo/globalshared/rest"
	"monorepo/services/user/internal/modules/user/domain"
	"monorepo/services/user/pkg/helper"
	shareddomain "monorepo/services/user/pkg/shared/domain"
	"monorepo/services/user/pkg/shared/usecase/common"

	"github.com/golangid/candi/candishared"
	"github.com/golangid/candi/tracer"
	"gorm.io/gorm"
)

const minPasswordLength = 8

func normalize(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func validStatus(s string) bool {
	return s == shareddomain.UserStatusActive || s == shareddomain.UserStatusDisabled
}

func (uc *userUsecaseImpl) GetAllUser(ctx context.Context, realm string, filter *domain.FilterUser) (data domain.ResponseUserList, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "UserUsecase:GetAllUser")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return data, err
	}
	users, err := uc.repoSQL.UserRepo().FetchAll(ctx, r.ID, filter)
	if err != nil {
		return data, err
	}
	data.Meta = candishared.NewMeta(filter.Page, filter.Limit, uc.repoSQL.UserRepo().Count(ctx, r.ID, filter))
	data.Data = make([]domain.ResponseUser, 0, len(users))
	for i := range users {
		var u domain.ResponseUser
		u.Serialize(&users[i])
		data.Data = append(data.Data, u)
	}
	return
}

func (uc *userUsecaseImpl) GetDetailUser(ctx context.Context, realm string, id int) (data domain.ResponseUser, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "UserUsecase:GetDetailUser")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return data, err
	}
	user, err := uc.repoSQL.UserRepo().Find(ctx, r.ID, id)
	if err != nil {
		return data, common.NotFound(err, "user")
	}
	data.Serialize(&user)
	roles, err := uc.repoSQL.UserRepo().Roles(ctx, user.ID)
	if err != nil {
		return data, err
	}
	data.Roles = roleRefs(roles)
	return
}

// checkUnique makes sure username / email / phone are not taken by another user of the realm
func (uc *userUsecaseImpl) checkUnique(ctx context.Context, realmID, selfID int, username, email, phone string) error {
	repo := uc.repoSQL.UserRepo()
	check := func(what string, u shareddomain.User, err error) error {
		if err == nil && u.ID != selfID {
			return rest.NewConflict(what + " is already used by another user of this realm")
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return nil
	}
	u, err := repo.FindByUsername(ctx, realmID, username)
	if err = check("username", u, err); err != nil {
		return err
	}
	if email != "" {
		u, err = repo.FindByEmail(ctx, realmID, email)
		if err = check("email", u, err); err != nil {
			return err
		}
	}
	if phone != "" {
		u, err = repo.FindByPhone(ctx, realmID, phone)
		if err = check("phone", u, err); err != nil {
			return err
		}
	}
	return nil
}

func (uc *userUsecaseImpl) CreateUser(ctx context.Context, realm string, req *domain.RequestCreateUser) (res domain.ResponseUser, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "UserUsecase:CreateUser")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return res, err
	}
	if len(req.Password) < minPasswordLength {
		return res, rest.NewInvalid("password must be at least 8 characters")
	}
	if req.Status == "" {
		req.Status = shareddomain.UserStatusActive
	}
	if !validStatus(req.Status) {
		return res, rest.NewInvalid("status must be active or disabled")
	}
	username, email, phone := normalize(req.Username), normalize(req.Email), strings.TrimSpace(req.Phone)
	if err = uc.checkUnique(ctx, r.ID, 0, username, email, phone); err != nil {
		return res, err
	}
	roleIDs := uniqueInts(req.RoleIDs)
	if len(roleIDs) > 0 && uc.repoSQL.UserRepo().CountRolesByIDs(ctx, r.ID, roleIDs) != len(roleIDs) {
		return res, rest.NewInvalid("one or more roles do not exist in realm " + realm)
	}
	hash, err := helper.HashSecret(req.Password)
	if err != nil {
		return res, err
	}
	user := shareddomain.User{
		RealmID: r.ID, Username: username, Email: helper.StrPtr(email), Phone: helper.StrPtr(phone),
		FullName: req.FullName, PasswordHash: hash, Status: req.Status,
	}
	err = uc.repoSQL.WithTransaction(ctx, func(ctx context.Context) error {
		if err := uc.repoSQL.UserRepo().Save(ctx, &user); err != nil {
			return err
		}
		return uc.repoSQL.UserRepo().ReplaceRoles(ctx, user.ID, roleIDs)
	})
	if err != nil {
		return res, err
	}
	res.Serialize(&user)
	return
}

func (uc *userUsecaseImpl) UpdateUser(ctx context.Context, realm string, id int, req *domain.RequestUpdateUser) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "UserUsecase:UpdateUser")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return err
	}
	user, err := uc.repoSQL.UserRepo().Find(ctx, r.ID, id)
	if err != nil {
		return common.NotFound(err, "user")
	}
	if user.IsServiceAccount {
		return rest.NewConflict("service accounts are managed through their client")
	}
	if req.Status == "" {
		req.Status = user.Status
	}
	if !validStatus(req.Status) {
		return rest.NewInvalid("status must be active or disabled")
	}
	username, email, phone := normalize(req.Username), normalize(req.Email), strings.TrimSpace(req.Phone)
	if err = uc.checkUnique(ctx, r.ID, user.ID, username, email, phone); err != nil {
		return err
	}
	disabling := user.Status != shareddomain.UserStatusDisabled && req.Status == shareddomain.UserStatusDisabled
	user.Username, user.Email, user.Phone = username, helper.StrPtr(email), helper.StrPtr(phone)
	user.FullName, user.Status = req.FullName, req.Status
	if req.Status == shareddomain.UserStatusActive {
		user.FailedAttempts, user.LockedUntil = 0, nil
	}
	return uc.repoSQL.WithTransaction(ctx, func(ctx context.Context) error {
		if err := uc.repoSQL.UserRepo().Save(ctx, &user); err != nil {
			return err
		}
		if disabling { // a disabled user must not keep working refresh tokens
			return uc.repoSQL.SessionRepo().RevokeByUser(ctx, user.ID)
		}
		return nil
	})
}

func (uc *userUsecaseImpl) DeleteUser(ctx context.Context, realm string, id int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "UserUsecase:DeleteUser")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return err
	}
	user, err := uc.repoSQL.UserRepo().Find(ctx, r.ID, id)
	if err != nil {
		return common.NotFound(err, "user")
	}
	if user.IsServiceAccount {
		return rest.NewConflict("service accounts are deleted together with their client")
	}
	return uc.repoSQL.WithTransaction(ctx, func(ctx context.Context) error {
		if err := uc.repoSQL.SessionRepo().RevokeByUser(ctx, user.ID); err != nil {
			return err
		}
		return uc.repoSQL.UserRepo().Delete(ctx, user.ID)
	})
}

func (uc *userUsecaseImpl) SetPassword(ctx context.Context, realm string, id int, password string) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "UserUsecase:SetPassword")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return err
	}
	if len(password) < minPasswordLength {
		return rest.NewInvalid("password must be at least 8 characters")
	}
	user, err := uc.repoSQL.UserRepo().Find(ctx, r.ID, id)
	if err != nil {
		return common.NotFound(err, "user")
	}
	if user.IsServiceAccount {
		return rest.NewConflict("service accounts authenticate with their client secret")
	}
	if user.PasswordHash, err = helper.HashSecret(password); err != nil {
		return err
	}
	user.FailedAttempts, user.LockedUntil = 0, nil
	return uc.repoSQL.WithTransaction(ctx, func(ctx context.Context) error {
		if err := uc.repoSQL.UserRepo().Save(ctx, &user); err != nil {
			return err
		}
		return uc.repoSQL.SessionRepo().RevokeByUser(ctx, user.ID) // log out everywhere after a password change
	})
}

func (uc *userUsecaseImpl) UnlockUser(ctx context.Context, realm string, id int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "UserUsecase:UnlockUser")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return err
	}
	user, err := uc.repoSQL.UserRepo().Find(ctx, r.ID, id)
	if err != nil {
		return common.NotFound(err, "user")
	}
	user.FailedAttempts, user.LockedUntil = 0, nil
	return uc.repoSQL.UserRepo().Save(ctx, &user)
}
