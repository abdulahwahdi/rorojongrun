package graphqlhandler

import (
	"context"

	"monorepo/services/user/internal/modules/client/domain"

	"github.com/golangid/candi/candihelper"
	"github.com/golangid/candi/candishared"
	"github.com/golangid/candi/tracer"
)

// GetAllClient resolver
func (q *GraphQLHandler) GetAllClient(ctx context.Context, input struct {
	Realm  string
	Filter *struct {
		candishared.NullableFilter
		domain.FilterClient
	}
}) (res domain.ResponseClientList, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "ClientDeliveryGraphQL:GetAllClient")
	defer trace.Finish()

	filter := candihelper.UnwrapPtr(input.Filter)
	filter.FilterClient.Filter = filter.ToFilter()
	if err := q.validator.ValidateDocument("client/get_all", filter.FilterClient); err != nil {
		return res, err
	}
	return q.uc.Client().GetAllClient(ctx, input.Realm, &filter.FilterClient)
}

// GetDetailClient resolver
func (q *GraphQLHandler) GetDetailClient(ctx context.Context, input struct {
	Realm string
	ID    int
}) (domain.ResponseClient, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "ClientDeliveryGraphQL:GetDetailClient")
	defer trace.Finish()

	return q.uc.Client().GetDetailClient(ctx, input.Realm, input.ID)
}
