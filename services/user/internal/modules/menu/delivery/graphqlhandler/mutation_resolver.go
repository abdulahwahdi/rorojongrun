package graphqlhandler

import (
	"context"

	"monorepo/services/user/internal/modules/menu/domain"

	"github.com/golangid/candi/tracer"
)

// CreateMenu resolver
func (m *GraphQLHandler) CreateMenu(ctx context.Context, input struct {
	Realm    string
	ClientID string
	Data     domain.RequestMenu
}) (domain.ResponseMenu, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "MenuDeliveryGraphQL:CreateMenu")
	defer trace.Finish()

	return m.uc.Menu().CreateMenu(ctx, input.Realm, input.ClientID, &input.Data)
}

// UpdateMenu resolver
func (m *GraphQLHandler) UpdateMenu(ctx context.Context, input struct {
	Realm    string
	ClientID string
	ID       int
	Data     domain.RequestMenu
}) (string, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "MenuDeliveryGraphQL:UpdateMenu")
	defer trace.Finish()

	if err := m.uc.Menu().UpdateMenu(ctx, input.Realm, input.ClientID, input.ID, &input.Data); err != nil {
		return "", err
	}
	return "Success", nil
}

// DeleteMenu resolver
func (m *GraphQLHandler) DeleteMenu(ctx context.Context, input struct {
	Realm    string
	ClientID string
	ID       int
}) (string, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "MenuDeliveryGraphQL:DeleteMenu")
	defer trace.Finish()

	if err := m.uc.Menu().DeleteMenu(ctx, input.Realm, input.ClientID, input.ID); err != nil {
		return "", err
	}
	return "Success", nil
}
