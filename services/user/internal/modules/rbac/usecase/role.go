package usecase

import (
	"context"

	"monorepo/services/user/internal/modules/rbac/domain"
	"monorepo/services/user/pkg/helper"
	shareddomain "monorepo/services/user/pkg/shared/domain"
	"monorepo/services/user/pkg/shared/usecase/common"

	"github.com/golangid/candi/candishared"
	"github.com/golangid/candi/tracer"
)

func (uc *rbacUsecaseImpl) GetAllRole(ctx context.Context, realm string, filter *domain.FilterRole) (data domain.ResponseRoleList, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RbacUsecase:GetAllRole")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return data, err
	}
	roles, err := uc.repoSQL.RoleRepo().FetchAll(ctx, r.ID, filter)
	if err != nil {
		return data, err
	}
	data.Meta = candishared.NewMeta(filter.Page, filter.Limit, uc.repoSQL.RoleRepo().Count(ctx, r.ID, filter))
	data.Data = make([]domain.ResponseRole, 0, len(roles))
	for i := range roles {
		var item domain.ResponseRole
		item.Serialize(&roles[i])
		data.Data = append(data.Data, item)
	}
	return
}

func (uc *rbacUsecaseImpl) GetDetailRole(ctx context.Context, realm string, id int) (data domain.ResponseRole, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RbacUsecase:GetDetailRole")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return data, err
	}
	role, err := uc.repoSQL.RoleRepo().Find(ctx, r.ID, id)
	if err != nil {
		return data, common.NotFound(err, "role")
	}
	data.Serialize(&role)
	perms, err := uc.repoSQL.RoleRepo().Permissions(ctx, role.ID)
	if err != nil {
		return data, err
	}
	data.Permissions = serializePermissions(perms)
	return
}

func (uc *rbacUsecaseImpl) CreateRole(ctx context.Context, realm string, req *domain.RequestRole) (res domain.ResponseRole, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RbacUsecase:CreateRole")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return res, err
	}
	if _, err = uc.repoSQL.RoleRepo().FindByName(ctx, r.ID, req.Name); err == nil {
		return res, helper.NewConflict("role " + req.Name + " already exists in realm " + realm)
	}
	role := shareddomain.Role{RealmID: r.ID, Name: req.Name, Description: req.Description}
	if err = uc.repoSQL.RoleRepo().Save(ctx, &role); err != nil {
		return res, err
	}
	res.Serialize(&role)
	return
}

func (uc *rbacUsecaseImpl) UpdateRole(ctx context.Context, realm string, id int, req *domain.RequestRole) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RbacUsecase:UpdateRole")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return err
	}
	role, err := uc.repoSQL.RoleRepo().Find(ctx, r.ID, id)
	if err != nil {
		return common.NotFound(err, "role")
	}
	if existing, findErr := uc.repoSQL.RoleRepo().FindByName(ctx, r.ID, req.Name); findErr == nil && existing.ID != role.ID {
		return helper.NewConflict("role " + req.Name + " already exists in realm " + realm)
	}
	role.Name, role.Description = req.Name, req.Description
	return uc.repoSQL.RoleRepo().Save(ctx, &role)
}

func (uc *rbacUsecaseImpl) DeleteRole(ctx context.Context, realm string, id int, force bool) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RbacUsecase:DeleteRole")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return err
	}
	role, err := uc.repoSQL.RoleRepo().Find(ctx, r.ID, id)
	if err != nil {
		return common.NotFound(err, "role")
	}
	if n := uc.repoSQL.RoleRepo().CountUsers(ctx, role.ID); n > 0 && !force {
		return helper.NewConflict("role is still assigned to users, remove the assignments or pass force=true")
	}
	return uc.repoSQL.WithTransaction(ctx, func(ctx context.Context) error {
		return uc.repoSQL.RoleRepo().Delete(ctx, role.ID)
	})
}

func (uc *rbacUsecaseImpl) GetRolePermissions(ctx context.Context, realm string, roleID int) (data []domain.ResponsePermission, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RbacUsecase:GetRolePermissions")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return nil, err
	}
	role, err := uc.repoSQL.RoleRepo().Find(ctx, r.ID, roleID)
	if err != nil {
		return nil, common.NotFound(err, "role")
	}
	perms, err := uc.repoSQL.RoleRepo().Permissions(ctx, role.ID)
	if err != nil {
		return nil, err
	}
	return serializePermissions(perms), nil
}

func (uc *rbacUsecaseImpl) AddRolePermission(ctx context.Context, realm string, roleID, permissionID int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RbacUsecase:AddRolePermission")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return err
	}
	role, err := uc.repoSQL.RoleRepo().Find(ctx, r.ID, roleID)
	if err != nil {
		return common.NotFound(err, "role")
	}
	perm, err := uc.repoSQL.PermissionRepo().Find(ctx, r.ID, permissionID)
	if err != nil {
		return common.NotFound(err, "permission")
	}
	return uc.repoSQL.RoleRepo().AddPermission(ctx, role.ID, perm.ID)
}

func (uc *rbacUsecaseImpl) ReplaceRolePermissions(ctx context.Context, realm string, roleID int, permissionIDs []int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RbacUsecase:ReplaceRolePermissions")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return err
	}
	role, err := uc.repoSQL.RoleRepo().Find(ctx, r.ID, roleID)
	if err != nil {
		return common.NotFound(err, "role")
	}
	permissionIDs = uniqueInts(permissionIDs)
	if uc.repoSQL.PermissionRepo().CountByIDs(ctx, r.ID, permissionIDs) != len(permissionIDs) {
		return helper.NewInvalid("one or more permissions do not exist in realm " + realm)
	}
	return uc.repoSQL.RoleRepo().ReplacePermissions(ctx, role.ID, permissionIDs)
}

func (uc *rbacUsecaseImpl) RemoveRolePermission(ctx context.Context, realm string, roleID, permissionID int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RbacUsecase:RemoveRolePermission")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return err
	}
	role, err := uc.repoSQL.RoleRepo().Find(ctx, r.ID, roleID)
	if err != nil {
		return common.NotFound(err, "role")
	}
	return uc.repoSQL.RoleRepo().RemovePermission(ctx, role.ID, permissionID)
}

func serializePermissions(perms []shareddomain.Permission) []domain.ResponsePermission {
	out := make([]domain.ResponsePermission, 0, len(perms))
	for i := range perms {
		var p domain.ResponsePermission
		p.Serialize(&perms[i])
		out = append(out, p)
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
