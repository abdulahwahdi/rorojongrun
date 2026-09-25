package graphqlhandler

import (
	"context"

	"monorepo/services/user/internal/modules/rbac/domain"

	"github.com/golangid/candi/tracer"
)

const ok = "Success"

// CreateRole resolver
func (m *GraphQLHandler) CreateRole(ctx context.Context, input struct {
	Realm string
	Data  domain.RequestRole
}) (domain.ResponseRole, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RbacDeliveryGraphQL:CreateRole")
	defer trace.Finish()

	return m.uc.Rbac().CreateRole(ctx, input.Realm, &input.Data)
}

// UpdateRole resolver
func (m *GraphQLHandler) UpdateRole(ctx context.Context, input struct {
	Realm string
	ID    int
	Data  domain.RequestRole
}) (string, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RbacDeliveryGraphQL:UpdateRole")
	defer trace.Finish()

	if err := m.uc.Rbac().UpdateRole(ctx, input.Realm, input.ID, &input.Data); err != nil {
		return "", err
	}
	return ok, nil
}

// DeleteRole resolver
func (m *GraphQLHandler) DeleteRole(ctx context.Context, input struct {
	Realm string
	ID    int
	Force *bool
}) (string, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RbacDeliveryGraphQL:DeleteRole")
	defer trace.Finish()

	force := input.Force != nil && *input.Force
	if err := m.uc.Rbac().DeleteRole(ctx, input.Realm, input.ID, force); err != nil {
		return "", err
	}
	return ok, nil
}

// AddRolePermission resolver
func (m *GraphQLHandler) AddRolePermission(ctx context.Context, input struct {
	Realm        string
	RoleID       int
	PermissionID int
}) (string, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RbacDeliveryGraphQL:AddRolePermission")
	defer trace.Finish()

	if err := m.uc.Rbac().AddRolePermission(ctx, input.Realm, input.RoleID, input.PermissionID); err != nil {
		return "", err
	}
	return ok, nil
}

// ReplaceRolePermissions resolver
func (m *GraphQLHandler) ReplaceRolePermissions(ctx context.Context, input struct {
	Realm         string
	RoleID        int
	PermissionIDs []int
}) (string, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RbacDeliveryGraphQL:ReplaceRolePermissions")
	defer trace.Finish()

	if err := m.uc.Rbac().ReplaceRolePermissions(ctx, input.Realm, input.RoleID, input.PermissionIDs); err != nil {
		return "", err
	}
	return ok, nil
}

// RemoveRolePermission resolver
func (m *GraphQLHandler) RemoveRolePermission(ctx context.Context, input struct {
	Realm        string
	RoleID       int
	PermissionID int
}) (string, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RbacDeliveryGraphQL:RemoveRolePermission")
	defer trace.Finish()

	if err := m.uc.Rbac().RemoveRolePermission(ctx, input.Realm, input.RoleID, input.PermissionID); err != nil {
		return "", err
	}
	return ok, nil
}

// CreatePermission resolver
func (m *GraphQLHandler) CreatePermission(ctx context.Context, input struct {
	Realm string
	Data  domain.RequestPermission
}) (domain.ResponsePermission, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RbacDeliveryGraphQL:CreatePermission")
	defer trace.Finish()

	return m.uc.Rbac().CreatePermission(ctx, input.Realm, &input.Data)
}

// UpdatePermission resolver
func (m *GraphQLHandler) UpdatePermission(ctx context.Context, input struct {
	Realm string
	ID    int
	Data  domain.RequestPermission
}) (string, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RbacDeliveryGraphQL:UpdatePermission")
	defer trace.Finish()

	if err := m.uc.Rbac().UpdatePermission(ctx, input.Realm, input.ID, &input.Data); err != nil {
		return "", err
	}
	return ok, nil
}

// DeletePermission resolver
func (m *GraphQLHandler) DeletePermission(ctx context.Context, input struct {
	Realm string
	ID    int
}) (string, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RbacDeliveryGraphQL:DeletePermission")
	defer trace.Finish()

	if err := m.uc.Rbac().DeletePermission(ctx, input.Realm, input.ID); err != nil {
		return "", err
	}
	return ok, nil
}

// UpsertPermissions resolver
func (m *GraphQLHandler) UpsertPermissions(ctx context.Context, input struct {
	Realm       string
	Permissions []domain.RequestPermission
}) ([]domain.ResponsePermission, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RbacDeliveryGraphQL:UpsertPermissions")
	defer trace.Finish()

	return m.uc.Rbac().UpsertPermissions(ctx, input.Realm, input.Permissions)
}
