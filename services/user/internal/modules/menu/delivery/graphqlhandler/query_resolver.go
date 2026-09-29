package graphqlhandler

import (
	"context"

	"monorepo/services/user/internal/modules/menu/domain"

	"github.com/golangid/candi/candihelper"
	"github.com/golangid/candi/candishared"
	"github.com/golangid/candi/tracer"
)

// GetAllMenu resolver
func (q *GraphQLHandler) GetAllMenu(ctx context.Context, input struct {
	Realm    string
	ClientID string
	Tree     *bool
	Filter   *struct {
		candishared.NullableFilter
		domain.FilterMenu
	}
}) (res domain.ResponseMenuList, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "MenuDeliveryGraphQL:GetAllMenu")
	defer trace.Finish()

	filter := candihelper.UnwrapPtr(input.Filter)
	filter.FilterMenu.Filter = filter.ToFilter()
	filter.FilterMenu.Tree = input.Tree != nil && *input.Tree
	if err := q.validator.ValidateDocument("menu/get_all", filter.FilterMenu); err != nil {
		return res, err
	}
	return q.uc.Menu().GetAllMenu(ctx, input.Realm, input.ClientID, &filter.FilterMenu)
}

// GetDetailMenu resolver
func (q *GraphQLHandler) GetDetailMenu(ctx context.Context, input struct {
	Realm    string
	ClientID string
	ID       int
}) (domain.ResponseMenu, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "MenuDeliveryGraphQL:GetDetailMenu")
	defer trace.Finish()

	return q.uc.Menu().GetDetailMenu(ctx, input.Realm, input.ClientID, input.ID)
}
