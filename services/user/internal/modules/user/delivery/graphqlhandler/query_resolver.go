package graphqlhandler

import (
	"context"

	"monorepo/services/user/internal/modules/user/domain"

	"github.com/golangid/candi/candihelper"
	"github.com/golangid/candi/candishared"
	"github.com/golangid/candi/tracer"
)

// GetAllUser resolver
func (q *GraphQLHandler) GetAllUser(ctx context.Context, input struct {
	Realm  string
	Filter *struct {
		candishared.NullableFilter
		domain.FilterUser
	}
}) (res domain.ResponseUserList, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "UserDeliveryGraphQL:GetAllUser")
	defer trace.Finish()

	filter := candihelper.UnwrapPtr(input.Filter)
	filter.FilterUser.Filter = filter.ToFilter()
	if err := q.validator.ValidateDocument("user/get_all", filter.FilterUser); err != nil {
		return res, err
	}
	return q.uc.User().GetAllUser(ctx, input.Realm, &filter.FilterUser)
}

// GetDetailUser resolver
func (q *GraphQLHandler) GetDetailUser(ctx context.Context, input struct {
	Realm string
	ID    int
}) (domain.ResponseUser, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "UserDeliveryGraphQL:GetDetailUser")
	defer trace.Finish()

	return q.uc.User().GetDetailUser(ctx, input.Realm, input.ID)
}

// GetUserRoles resolver
func (q *GraphQLHandler) GetUserRoles(ctx context.Context, input struct {
	Realm  string
	UserID int
}) ([]domain.ResponseRoleRef, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "UserDeliveryGraphQL:GetUserRoles")
	defer trace.Finish()

	return q.uc.User().GetUserRoles(ctx, input.Realm, input.UserID)
}
