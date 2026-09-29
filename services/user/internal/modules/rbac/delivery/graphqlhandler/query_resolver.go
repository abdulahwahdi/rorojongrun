package graphqlhandler

import (
	"context"

	"monorepo/services/user/internal/modules/rbac/domain"

	"github.com/golangid/candi/candihelper"
	"github.com/golangid/candi/candishared"
	"github.com/golangid/candi/tracer"
)

// GetAllRole resolver
func (q *GraphQLHandler) GetAllRole(ctx context.Context, input struct {
	Realm  string
	Filter *struct {
		candishared.NullableFilter
		domain.FilterRole
	}
}) (res domain.ResponseRoleList, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RbacDeliveryGraphQL:GetAllRole")
	defer trace.Finish()

	filter := candihelper.UnwrapPtr(input.Filter)
	filter.FilterRole.Filter = filter.ToFilter()
	if err := q.validator.ValidateDocument("rbac/get_all_role", filter.FilterRole); err != nil {
		return res, err
	}
	return q.uc.Rbac().GetAllRole(ctx, input.Realm, &filter.FilterRole)
}

// GetDetailRole resolver
func (q *GraphQLHandler) GetDetailRole(ctx context.Context, input struct {
	Realm string
	ID    int
}) (domain.ResponseRole, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RbacDeliveryGraphQL:GetDetailRole")
	defer trace.Finish()

	return q.uc.Rbac().GetDetailRole(ctx, input.Realm, input.ID)
}

// GetRolePermissions resolver
func (q *GraphQLHandler) GetRolePermissions(ctx context.Context, input struct {
	Realm  string
	RoleID int
}) ([]domain.ResponsePermission, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RbacDeliveryGraphQL:GetRolePermissions")
	defer trace.Finish()

	return q.uc.Rbac().GetRolePermissions(ctx, input.Realm, input.RoleID)
}

// GetAllPermission resolver
func (q *GraphQLHandler) GetAllPermission(ctx context.Context, input struct {
	Realm  string
	Filter *struct {
		candishared.NullableFilter
		domain.FilterPermission
	}
}) (res domain.ResponsePermissionList, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RbacDeliveryGraphQL:GetAllPermission")
	defer trace.Finish()

	filter := candihelper.UnwrapPtr(input.Filter)
	filter.FilterPermission.Filter = filter.ToFilter()
	if err := q.validator.ValidateDocument("rbac/get_all_permission", filter.FilterPermission); err != nil {
		return res, err
	}
	return q.uc.Rbac().GetAllPermission(ctx, input.Realm, &filter.FilterPermission)
}

// GetDetailPermission resolver
func (q *GraphQLHandler) GetDetailPermission(ctx context.Context, input struct {
	Realm string
	ID    int
}) (domain.ResponsePermission, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RbacDeliveryGraphQL:GetDetailPermission")
	defer trace.Finish()

	return q.uc.Rbac().GetDetailPermission(ctx, input.Realm, input.ID)
}
