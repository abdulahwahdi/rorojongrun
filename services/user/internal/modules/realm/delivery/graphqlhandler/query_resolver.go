package graphqlhandler

import (
	"context"

	"monorepo/services/user/internal/modules/realm/domain"

	"github.com/golangid/candi/candihelper"
	"github.com/golangid/candi/candishared"
	"github.com/golangid/candi/tracer"
)

// GetAllRealm resolver
func (q *GraphQLHandler) GetAllRealm(ctx context.Context, input struct {
	Filter *struct {
		candishared.NullableFilter
		domain.FilterRealm
	}
}) (res domain.ResponseRealmList, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmDeliveryGraphQL:GetAllRealm")
	defer trace.Finish()

	filter := candihelper.UnwrapPtr(input.Filter)
	filter.FilterRealm.Filter = filter.ToFilter()
	if err := q.validator.ValidateDocument("realm/get_all", filter.FilterRealm); err != nil {
		return res, err
	}
	return q.uc.Realm().GetAllRealm(ctx, &filter.FilterRealm)
}

// GetDetailRealm resolver
func (q *GraphQLHandler) GetDetailRealm(ctx context.Context, input struct{ Name string }) (domain.ResponseRealm, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmDeliveryGraphQL:GetDetailRealm")
	defer trace.Finish()

	return q.uc.Realm().GetDetailRealm(ctx, input.Name)
}

// GetAllRealmKey resolver
func (q *GraphQLHandler) GetAllRealmKey(ctx context.Context, input struct {
	Realm  string
	Filter *struct {
		candishared.NullableFilter
		domain.FilterRealmKey
	}
}) (res domain.ResponseRealmKeyList, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmDeliveryGraphQL:GetAllRealmKey")
	defer trace.Finish()

	filter := candihelper.UnwrapPtr(input.Filter)
	filter.FilterRealmKey.Filter = filter.ToFilter()
	if err := q.validator.ValidateDocument("realm/get_all_key", filter.FilterRealmKey); err != nil {
		return res, err
	}
	return q.uc.Realm().GetAllRealmKey(ctx, input.Realm, &filter.FilterRealmKey)
}

// GetDetailRealmKey resolver
func (q *GraphQLHandler) GetDetailRealmKey(ctx context.Context, input struct {
	Realm string
	ID    int
}) (domain.ResponseRealmKey, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmDeliveryGraphQL:GetDetailRealmKey")
	defer trace.Finish()

	return q.uc.Realm().GetDetailRealmKey(ctx, input.Realm, input.ID)
}
