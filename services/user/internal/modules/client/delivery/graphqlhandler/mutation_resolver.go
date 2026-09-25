package graphqlhandler

import (
	"context"

	"monorepo/services/user/internal/modules/client/domain"

	"github.com/golangid/candi/tracer"
)

// CreateClient resolver
func (m *GraphQLHandler) CreateClient(ctx context.Context, input struct {
	Realm string
	Data  domain.RequestCreateClient
}) (domain.ResponseClient, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "ClientDeliveryGraphQL:CreateClient")
	defer trace.Finish()

	return m.uc.Client().CreateClient(ctx, input.Realm, &input.Data)
}

// UpdateClient resolver
func (m *GraphQLHandler) UpdateClient(ctx context.Context, input struct {
	Realm string
	ID    int
	Data  domain.RequestUpdateClient
}) (string, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "ClientDeliveryGraphQL:UpdateClient")
	defer trace.Finish()

	if err := m.uc.Client().UpdateClient(ctx, input.Realm, input.ID, &input.Data); err != nil {
		return "", err
	}
	return "Success", nil
}

// DeleteClient resolver
func (m *GraphQLHandler) DeleteClient(ctx context.Context, input struct {
	Realm string
	ID    int
}) (string, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "ClientDeliveryGraphQL:DeleteClient")
	defer trace.Finish()

	if err := m.uc.Client().DeleteClient(ctx, input.Realm, input.ID); err != nil {
		return "", err
	}
	return "Success", nil
}

// RotateClientSecret resolver
func (m *GraphQLHandler) RotateClientSecret(ctx context.Context, input struct {
	Realm string
	ID    int
}) (domain.ResponseClient, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "ClientDeliveryGraphQL:RotateClientSecret")
	defer trace.Finish()

	return m.uc.Client().RotateClientSecret(ctx, input.Realm, input.ID)
}
