package usecase

import (
	"context"
	"errors"

	"monorepo/services/user/internal/modules/rbac/domain"
	"monorepo/services/user/pkg/helper"
	shareddomain "monorepo/services/user/pkg/shared/domain"
	"monorepo/services/user/pkg/shared/usecase/common"

	"github.com/golangid/candi/candishared"
	"github.com/golangid/candi/tracer"
	"gorm.io/gorm"
)

func validPermissionType(t string) string {
	if t == "" {
		return shareddomain.PermissionTypeAPI
	}
	return t
}

func (uc *rbacUsecaseImpl) GetAllPermission(ctx context.Context, realm string, filter *domain.FilterPermission) (data domain.ResponsePermissionList, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RbacUsecase:GetAllPermission")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return data, err
	}
	perms, err := uc.repoSQL.PermissionRepo().FetchAll(ctx, r.ID, filter)
	if err != nil {
		return data, err
	}
	data.Meta = candishared.NewMeta(filter.Page, filter.Limit, uc.repoSQL.PermissionRepo().Count(ctx, r.ID, filter))
	data.Data = serializePermissions(perms)
	return
}

func (uc *rbacUsecaseImpl) GetDetailPermission(ctx context.Context, realm string, id int) (data domain.ResponsePermission, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RbacUsecase:GetDetailPermission")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return data, err
	}
	perm, err := uc.repoSQL.PermissionRepo().Find(ctx, r.ID, id)
	if err != nil {
		return data, common.NotFound(err, "permission")
	}
	data.Serialize(&perm)
	return
}

func (uc *rbacUsecaseImpl) CreatePermission(ctx context.Context, realm string, req *domain.RequestPermission) (res domain.ResponsePermission, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RbacUsecase:CreatePermission")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return res, err
	}
	if _, err = uc.repoSQL.PermissionRepo().FindByServiceCode(ctx, r.ID, req.Service, req.Code); err == nil {
		return res, helper.NewConflict("permission " + req.Service + ":" + req.Code + " already exists in realm " + realm)
	}
	perm := shareddomain.Permission{
		RealmID: r.ID, Service: req.Service, Code: req.Code, Type: validPermissionType(req.Type), Description: req.Description,
	}
	if err = uc.repoSQL.PermissionRepo().Save(ctx, &perm); err != nil {
		return res, err
	}
	res.Serialize(&perm)
	return
}

func (uc *rbacUsecaseImpl) UpdatePermission(ctx context.Context, realm string, id int, req *domain.RequestPermission) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RbacUsecase:UpdatePermission")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return err
	}
	perm, err := uc.repoSQL.PermissionRepo().Find(ctx, r.ID, id)
	if err != nil {
		return common.NotFound(err, "permission")
	}
	if existing, findErr := uc.repoSQL.PermissionRepo().FindByServiceCode(ctx, r.ID, req.Service, req.Code); findErr == nil && existing.ID != perm.ID {
		return helper.NewConflict("permission " + req.Service + ":" + req.Code + " already exists in realm " + realm)
	}
	perm.Service, perm.Code, perm.Type, perm.Description = req.Service, req.Code, validPermissionType(req.Type), req.Description
	return uc.repoSQL.PermissionRepo().Save(ctx, &perm)
}

func (uc *rbacUsecaseImpl) DeletePermission(ctx context.Context, realm string, id int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RbacUsecase:DeletePermission")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return err
	}
	perm, err := uc.repoSQL.PermissionRepo().Find(ctx, r.ID, id)
	if err != nil {
		return common.NotFound(err, "permission")
	}
	return uc.repoSQL.WithTransaction(ctx, func(ctx context.Context) error {
		return uc.repoSQL.PermissionRepo().Delete(ctx, perm.ID)
	})
}

func (uc *rbacUsecaseImpl) UpsertPermissions(ctx context.Context, realm string, reqs []domain.RequestPermission) (res []domain.ResponsePermission, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RbacUsecase:UpsertPermissions")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return nil, err
	}
	saved := make([]shareddomain.Permission, 0, len(reqs))
	err = uc.repoSQL.WithTransaction(ctx, func(ctx context.Context) error {
		for _, req := range reqs {
			perm, findErr := uc.repoSQL.PermissionRepo().FindByServiceCode(ctx, r.ID, req.Service, req.Code)
			if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
				return findErr
			}
			if findErr != nil { // not found -> create
				perm = shareddomain.Permission{RealmID: r.ID, Service: req.Service, Code: req.Code}
			}
			perm.Type, perm.Description = validPermissionType(req.Type), req.Description
			if err := uc.repoSQL.PermissionRepo().Save(ctx, &perm); err != nil {
				return err
			}
			saved = append(saved, perm)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return serializePermissions(saved), nil
}
