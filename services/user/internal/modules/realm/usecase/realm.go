package usecase

import (
	"context"
	"regexp"

	"monorepo/globalshared/rest"
	"monorepo/services/user/internal/modules/realm/domain"
	shareddomain "monorepo/services/user/pkg/shared/domain"
	"monorepo/services/user/pkg/shared/usecase/common"

	"github.com/golangid/candi/candishared"
	"github.com/golangid/candi/tracer"
)

var realmNamePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

func (uc *realmUsecaseImpl) GetAllRealm(ctx context.Context, filter *domain.FilterRealm) (data domain.ResponseRealmList, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmUsecase:GetAllRealm")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	if err = common.RequireMaster(ctx); err != nil {
		return
	}
	realms, err := uc.repoSQL.RealmRepo().FetchAll(ctx, filter)
	if err != nil {
		return data, err
	}
	data.Meta = candishared.NewMeta(filter.Page, filter.Limit, uc.repoSQL.RealmRepo().Count(ctx, filter))
	data.Data = make([]domain.ResponseRealm, 0, len(realms))
	for i := range realms {
		var r domain.ResponseRealm
		r.Serialize(&realms[i])
		data.Data = append(data.Data, r)
	}
	return
}

func (uc *realmUsecaseImpl) GetDetailRealm(ctx context.Context, name string) (data domain.ResponseRealm, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmUsecase:GetDetailRealm")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	realm, err := common.ResolveRealm(ctx, uc.repoSQL, name)
	if err != nil {
		return data, err
	}
	data.Serialize(&realm)
	return
}

func (uc *realmUsecaseImpl) CreateRealm(ctx context.Context, req *domain.RequestRealm) (res domain.ResponseRealm, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmUsecase:CreateRealm")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	if err = common.RequireMaster(ctx); err != nil {
		return
	}
	if !realmNamePattern.MatchString(req.Name) {
		return res, rest.NewInvalid("realm name must be a lowercase slug (a-z, 0-9, '-')")
	}
	if _, err = uc.repoSQL.RealmRepo().FindByName(ctx, req.Name); err == nil {
		return res, rest.NewConflict("realm " + req.Name + " already exists")
	}

	realm := shareddomain.Realm{
		Name: req.Name, Enabled: true,
		AccessTokenTTLSec: 900, RefreshTokenTTLSec: 30 * 24 * 3600, MaxFailedAttempts: 5, LockoutSec: 900,
	}
	if err = applyRealmRequest(&realm, req); err != nil {
		return res, err
	}

	err = uc.repoSQL.WithTransaction(ctx, func(ctx context.Context) error {
		if err := uc.repoSQL.RealmRepo().Save(ctx, &realm); err != nil {
			return err
		}
		if _, err := uc.newSigningKey(ctx, realm.ID); err != nil {
			return err
		}
		return uc.seedRealmAdminRole(ctx, realm.ID)
	})
	if err != nil {
		return res, err
	}
	res.Serialize(&realm)
	return
}

// seedRealmAdminRole gives every realm a "realm-admin" role allowed to use every API of this service
func (uc *realmUsecaseImpl) seedRealmAdminRole(ctx context.Context, realmID int) error {
	perm := shareddomain.Permission{
		RealmID: realmID, Service: common.AdminService, Code: shareddomain.WildcardPermission,
		Type: shareddomain.PermissionTypeAPI, Description: "All APIs of the user service within this realm",
	}
	if err := uc.repoSQL.PermissionRepo().Save(ctx, &perm); err != nil {
		return err
	}
	role := shareddomain.Role{RealmID: realmID, Name: "realm-admin", Description: "Manages users, roles, clients and menus of this realm"}
	if err := uc.repoSQL.RoleRepo().Save(ctx, &role); err != nil {
		return err
	}
	return uc.repoSQL.RoleRepo().ReplacePermissions(ctx, role.ID, []int{perm.ID})
}

func applyRealmRequest(realm *shareddomain.Realm, req *domain.RequestRealm) error {
	if req.DisplayName != nil {
		realm.DisplayName = *req.DisplayName
	}
	if req.Enabled != nil {
		realm.Enabled = *req.Enabled
	}
	if req.AccessTokenTTLSec != nil {
		realm.AccessTokenTTLSec = *req.AccessTokenTTLSec
	}
	if req.RefreshTokenTTLSec != nil {
		realm.RefreshTokenTTLSec = *req.RefreshTokenTTLSec
	}
	if req.MaxFailedAttempts != nil {
		realm.MaxFailedAttempts = *req.MaxFailedAttempts
	}
	if req.LockoutSec != nil {
		realm.LockoutSec = *req.LockoutSec
	}
	if req.OTPLoginEnabled != nil {
		realm.OTPLoginEnabled = *req.OTPLoginEnabled
	}
	if realm.AccessTokenTTLSec <= 0 || realm.RefreshTokenTTLSec < realm.AccessTokenTTLSec {
		return rest.NewInvalid("refresh token ttl must be >= access token ttl, both positive")
	}
	return nil
}

func (uc *realmUsecaseImpl) UpdateRealm(ctx context.Context, name string, req *domain.RequestRealm) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmUsecase:UpdateRealm")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	realm, err := common.ResolveRealm(ctx, uc.repoSQL, name)
	if err != nil {
		return err
	}
	if realm.Name == common.MasterRealm && req.Enabled != nil && !*req.Enabled {
		return rest.NewInvalid("the master realm cannot be disabled")
	}
	if err = applyRealmRequest(&realm, req); err != nil {
		return err
	}
	return uc.repoSQL.RealmRepo().Save(ctx, &realm)
}

func (uc *realmUsecaseImpl) DeleteRealm(ctx context.Context, name string) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmUsecase:DeleteRealm")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	if err = common.RequireMaster(ctx); err != nil {
		return
	}
	if name == common.MasterRealm {
		return rest.NewInvalid("the master realm cannot be deleted")
	}
	realm, err := common.LoadRealm(ctx, uc.repoSQL, name)
	if err != nil {
		return err
	}
	return uc.repoSQL.WithTransaction(ctx, func(ctx context.Context) error {
		// revoke every session so already issued refresh tokens die with the realm
		if err := uc.repoSQL.SessionRepo().RevokeByRealm(ctx, realm.ID); err != nil {
			return err
		}
		return uc.repoSQL.RealmRepo().Delete(ctx, realm.ID)
	})
}
