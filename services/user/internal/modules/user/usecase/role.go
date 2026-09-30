package usecase

import (
	"context"

	"monorepo/globalshared/rest"
	"monorepo/services/user/internal/modules/user/domain"
	shareddomain "monorepo/services/user/pkg/shared/domain"
	"monorepo/services/user/pkg/shared/usecase/common"

	"github.com/golangid/candi/tracer"
)

func roleRefs(roles []shareddomain.Role) []domain.ResponseRoleRef {
	out := make([]domain.ResponseRoleRef, 0, len(roles))
	for _, r := range roles {
		out = append(out, domain.ResponseRoleRef{ID: r.ID, Name: r.Name, Description: r.Description})
	}
	return out
}

func uniqueInts(in []int) []int {
	seen := make(map[int]struct{}, len(in))
	out := make([]int, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}
	return out
}

// loadUser authorizes the realm and loads the user, shared by the role assignment methods
func (uc *userUsecaseImpl) loadUser(ctx context.Context, realm string, userID int) (r shareddomain.Realm, user shareddomain.User, err error) {
	if r, err = common.ResolveRealm(ctx, uc.repoSQL, realm); err != nil {
		return
	}
	if user, err = uc.repoSQL.UserRepo().Find(ctx, r.ID, userID); err != nil {
		err = common.NotFound(err, "user")
	}
	return
}

func (uc *userUsecaseImpl) GetUserRoles(ctx context.Context, realm string, userID int) (data []domain.ResponseRoleRef, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "UserUsecase:GetUserRoles")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	_, user, err := uc.loadUser(ctx, realm, userID)
	if err != nil {
		return nil, err
	}
	roles, err := uc.repoSQL.UserRepo().Roles(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	return roleRefs(roles), nil
}

func (uc *userUsecaseImpl) AddUserRole(ctx context.Context, realm string, userID, roleID int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "UserUsecase:AddUserRole")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, user, err := uc.loadUser(ctx, realm, userID)
	if err != nil {
		return err
	}
	role, err := uc.repoSQL.RoleRepo().Find(ctx, r.ID, roleID)
	if err != nil {
		return common.NotFound(err, "role")
	}
	return uc.repoSQL.UserRepo().AddRole(ctx, user.ID, role.ID)
}

func (uc *userUsecaseImpl) ReplaceUserRoles(ctx context.Context, realm string, userID int, roleIDs []int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "UserUsecase:ReplaceUserRoles")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, user, err := uc.loadUser(ctx, realm, userID)
	if err != nil {
		return err
	}
	roleIDs = uniqueInts(roleIDs)
	if len(roleIDs) > 0 && uc.repoSQL.UserRepo().CountRolesByIDs(ctx, r.ID, roleIDs) != len(roleIDs) {
		return rest.NewInvalid("one or more roles do not exist in realm " + realm)
	}
	return uc.repoSQL.UserRepo().ReplaceRoles(ctx, user.ID, roleIDs)
}

func (uc *userUsecaseImpl) RemoveUserRole(ctx context.Context, realm string, userID, roleID int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "UserUsecase:RemoveUserRole")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	_, user, err := uc.loadUser(ctx, realm, userID)
	if err != nil {
		return err
	}
	return uc.repoSQL.UserRepo().RemoveRole(ctx, user.ID, roleID)
}
