package graphqlhandler

import (
	"context"

	"monorepo/services/user/internal/modules/realm/domain"

	"github.com/golangid/candi/tracer"
)

// CreateRealm resolver
func (m *GraphQLHandler) CreateRealm(ctx context.Context, input struct{ Data domain.RequestRealm }) (domain.ResponseRealm, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmDeliveryGraphQL:CreateRealm")
	defer trace.Finish()

	return m.uc.Realm().CreateRealm(ctx, &input.Data)
}

// UpdateRealm resolver
func (m *GraphQLHandler) UpdateRealm(ctx context.Context, input struct {
	Name string
	Data domain.RequestRealm
}) (string, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmDeliveryGraphQL:UpdateRealm")
	defer trace.Finish()

	if err := m.uc.Realm().UpdateRealm(ctx, input.Name, &input.Data); err != nil {
		return "", err
	}
	return "Success", nil
}

// DeleteRealm resolver
func (m *GraphQLHandler) DeleteRealm(ctx context.Context, input struct{ Name string }) (string, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmDeliveryGraphQL:DeleteRealm")
	defer trace.Finish()

	if err := m.uc.Realm().DeleteRealm(ctx, input.Name); err != nil {
		return "", err
	}
	return "Success", nil
}

// RotateRealmKey resolver
func (m *GraphQLHandler) RotateRealmKey(ctx context.Context, input struct{ Realm string }) (domain.ResponseRealmKey, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmDeliveryGraphQL:RotateRealmKey")
	defer trace.Finish()

	return m.uc.Realm().RotateRealmKey(ctx, input.Realm)
}

// UpdateRealmKey resolver
func (m *GraphQLHandler) UpdateRealmKey(ctx context.Context, input struct {
	Realm string
	ID    int
	Data  domain.RequestRealmKey
}) (string, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmDeliveryGraphQL:UpdateRealmKey")
	defer trace.Finish()

	if err := m.uc.Realm().UpdateRealmKey(ctx, input.Realm, input.ID, &input.Data); err != nil {
		return "", err
	}
	return "Success", nil
}

// DeleteRealmKey resolver
func (m *GraphQLHandler) DeleteRealmKey(ctx context.Context, input struct {
	Realm string
	ID    int
}) (string, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmDeliveryGraphQL:DeleteRealmKey")
	defer trace.Finish()

	if err := m.uc.Realm().DeleteRealmKey(ctx, input.Realm, input.ID); err != nil {
		return "", err
	}
	return "Success", nil
}
