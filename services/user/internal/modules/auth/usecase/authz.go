package usecase

import (
	"context"
	"strconv"

	"monorepo/services/user/internal/modules/auth/domain"
	"monorepo/services/user/pkg/helper"
	shareddomain "monorepo/services/user/pkg/shared/domain"
	"monorepo/services/user/pkg/shared/usecase/common"

	"github.com/golangid/candi/tracer"
)

// CheckPermission decides whether a principal may use a permission code of a service. It is the single
// place where "is this allowed" is answered, for this service's own middleware and for the gRPC API
// every other service calls. A denial is (false, "", nil), only infrastructure problems are errors.
func (uc *authUsecaseImpl) CheckPermission(ctx context.Context, req domain.CheckPermissionRequest) (allowed bool, role string, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "AuthUsecase:CheckPermission")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()
	trace.SetTag("realm", req.Realm)
	trace.SetTag("permission", req.Service+":"+req.Code)

	realm, err := common.LoadRealm(ctx, uc.repoSQL, req.Realm)
	if err != nil {
		return false, "", nil // unknown realm: nobody is allowed
	}
	userID, convErr := strconv.Atoi(req.UserID)
	if convErr != nil || !realm.Enabled {
		return false, "", nil
	}
	user, err := uc.repoSQL.UserRepo().Find(ctx, realm.ID, userID)
	if err != nil || user.Status == shareddomain.UserStatusDisabled {
		return false, "", nil
	}
	if req.SessionID > 0 {
		active, err := uc.repoSQL.SessionRepo().IsFamilyActive(ctx, req.SessionID)
		if err != nil {
			return false, "", err
		}
		if !active {
			return false, "", nil
		}
	}
	perms, err := uc.repoSQL.PermissionRepo().EffectiveForUser(ctx, realm.ID, user.ID)
	if err != nil {
		return false, "", err
	}
	granted := make([][2]string, 0, len(perms))
	for _, p := range perms {
		granted = append(granted, [2]string{p.Service, p.Code})
	}
	if !helper.PermissionAllows(granted, req.Service, req.Code) {
		return false, "", nil
	}
	names, err := uc.roleNames(ctx, user.ID)
	if err != nil {
		return false, "", err
	}
	return true, joinNames(names), nil
}

func joinNames(names []string) string {
	out := ""
	for i, n := range names {
		if i > 0 {
			out += ","
		}
		out += n
	}
	return out
}

func (uc *authUsecaseImpl) GetUserPermissions(ctx context.Context, realm string, userID int, service string) (codes []string, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "AuthUsecase:GetUserPermissions")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.LoadRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return nil, err
	}
	perms, err := uc.repoSQL.PermissionRepo().EffectiveForUser(ctx, r.ID, userID)
	if err != nil {
		return nil, err
	}
	codes = []string{}
	for _, p := range perms {
		if p.Service == service || p.Service == shareddomain.WildcardPermission {
			codes = append(codes, p.Code)
		}
	}
	return codes, nil
}

func (uc *authUsecaseImpl) GetUserForService(ctx context.Context, realm string, userID int) (user shareddomain.User, roles []string, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "AuthUsecase:GetUserForService")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.LoadRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return user, nil, err
	}
	if user, err = uc.repoSQL.UserRepo().Find(ctx, r.ID, userID); err != nil {
		return user, nil, common.NotFound(err, "user")
	}
	roles, err = uc.roleNames(ctx, user.ID)
	return
}
