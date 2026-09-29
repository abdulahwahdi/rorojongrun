package usecase

import (
	"context"
	"sort"
	"strconv"

	"monorepo/services/user/internal/modules/auth/domain"
	menudomain "monorepo/services/user/internal/modules/menu/domain"
	"monorepo/services/user/pkg/helper"
	"monorepo/services/user/pkg/shared/usecase/common"

	"github.com/golangid/candi/tracer"
)

func scanInt(s string, out *int) error {
	n, err := strconv.Atoi(s)
	*out = n
	return err
}

func (uc *authUsecaseImpl) roleNames(ctx context.Context, userID int) ([]string, error) {
	roles, err := uc.repoSQL.UserRepo().Roles(ctx, userID)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(roles))
	for _, r := range roles {
		names = append(names, r.Name)
	}
	return names, nil
}

func (uc *authUsecaseImpl) GetMe(ctx context.Context, realm string) (data domain.ResponseMe, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "AuthUsecase:GetMe")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, user, _, err := uc.authenticate(ctx, realm)
	if err != nil {
		return data, err
	}
	roles, err := uc.roleNames(ctx, user.ID)
	if err != nil {
		return data, err
	}
	return domain.ResponseMe{
		ID: user.ID, Realm: r.Name, Username: user.Username, Email: helper.StrVal(user.Email), Phone: helper.StrVal(user.Phone),
		FullName: user.FullName, IsServiceAccount: user.IsServiceAccount, Roles: roles,
	}, nil
}

func (uc *authUsecaseImpl) GetMyPermissions(ctx context.Context, realm, clientID string) (data domain.ResponseMyPermissions, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "AuthUsecase:GetMyPermissions")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, user, _, err := uc.authenticate(ctx, realm)
	if err != nil {
		return data, err
	}
	perms, err := uc.repoSQL.PermissionRepo().EffectiveForUser(ctx, r.ID, user.ID)
	if err != nil {
		return data, err
	}
	sort.Slice(perms, func(i, j int) bool {
		if perms[i].Service != perms[j].Service {
			return perms[i].Service < perms[j].Service
		}
		return perms[i].Code < perms[j].Code
	})
	roles, err := uc.roleNames(ctx, user.ID)
	if err != nil {
		return data, err
	}
	data = domain.ResponseMyPermissions{
		Realm: r.Name, Roles: roles,
		Permissions: make([]domain.ResponsePermissionRef, 0, len(perms)), Menus: []menudomain.ResponseMenu{},
	}
	for _, p := range perms {
		data.Permissions = append(data.Permissions, domain.ResponsePermissionRef{Service: p.Service, Code: p.Code, Type: p.Type})
	}
	if clientID != "" {
		client, err := uc.repoSQL.ClientRepo().FindByClientID(ctx, r.ID, clientID)
		if err != nil {
			return data, common.NotFound(err, "client")
		}
		if data.Menus, err = uc.sharedUsecase.GetUserMenu(ctx, r.ID, client.ID, perms); err != nil {
			return data, err
		}
	}
	return data, nil
}
