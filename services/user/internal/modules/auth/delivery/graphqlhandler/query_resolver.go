package graphqlhandler

import (
	"context"

	"monorepo/services/user/internal/modules/auth/domain"

	"github.com/golangid/candi/candihelper"
	"github.com/golangid/candi/candishared"
	"github.com/golangid/candi/tracer"
)

// GetMe resolver
func (q *GraphQLHandler) GetMe(ctx context.Context, input struct{ Realm string }) (domain.ResponseMe, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "AuthDeliveryGraphQL:GetMe")
	defer trace.Finish()

	return q.uc.Auth().GetMe(ctx, input.Realm)
}

// GetMyPermissions resolver
func (q *GraphQLHandler) GetMyPermissions(ctx context.Context, input struct {
	Realm    string
	ClientID *string
}) (domain.ResponseMyPermissions, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "AuthDeliveryGraphQL:GetMyPermissions")
	defer trace.Finish()

	return q.uc.Auth().GetMyPermissions(ctx, input.Realm, candihelper.PtrToString(input.ClientID))
}

// GetMySessions resolver
func (q *GraphQLHandler) GetMySessions(ctx context.Context, input struct {
	Realm  string
	Filter *struct {
		candishared.NullableFilter
		domain.FilterSession
	}
}) (res domain.ResponseSessionList, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "AuthDeliveryGraphQL:GetMySessions")
	defer trace.Finish()

	filter := candihelper.UnwrapPtr(input.Filter)
	filter.FilterSession.Filter = filter.ToFilter()
	return q.uc.Auth().GetMySessions(ctx, input.Realm, &filter.FilterSession)
}

// GetAllSession resolver
func (q *GraphQLHandler) GetAllSession(ctx context.Context, input struct {
	Realm  string
	Filter *struct {
		candishared.NullableFilter
		domain.FilterSession
	}
}) (res domain.ResponseSessionList, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "AuthDeliveryGraphQL:GetAllSession")
	defer trace.Finish()

	filter := candihelper.UnwrapPtr(input.Filter)
	filter.FilterSession.Filter = filter.ToFilter()
	if err := q.validator.ValidateDocument("auth/get_all_session", filter.FilterSession); err != nil {
		return res, err
	}
	return q.uc.Auth().GetAllSession(ctx, input.Realm, &filter.FilterSession)
}

// GetDetailSession resolver
func (q *GraphQLHandler) GetDetailSession(ctx context.Context, input struct {
	Realm string
	ID    int
}) (domain.ResponseSession, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "AuthDeliveryGraphQL:GetDetailSession")
	defer trace.Finish()

	return q.uc.Auth().GetDetailSession(ctx, input.Realm, input.ID)
}
