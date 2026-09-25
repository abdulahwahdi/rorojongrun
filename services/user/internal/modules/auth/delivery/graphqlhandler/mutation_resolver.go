package graphqlhandler

import (
	"context"

	"github.com/golangid/candi/tracer"
)

// Logout resolver
func (m *GraphQLHandler) Logout(ctx context.Context, input struct{ Realm string }) (string, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "AuthDeliveryGraphQL:Logout")
	defer trace.Finish()

	if err := m.uc.Auth().Logout(ctx, input.Realm); err != nil {
		return "", err
	}
	return "Success", nil
}

// RevokeMySession resolver
func (m *GraphQLHandler) RevokeMySession(ctx context.Context, input struct {
	Realm string
	ID    int
}) (string, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "AuthDeliveryGraphQL:RevokeMySession")
	defer trace.Finish()

	if err := m.uc.Auth().RevokeMySession(ctx, input.Realm, input.ID); err != nil {
		return "", err
	}
	return "Success", nil
}

// RevokeSession resolver
func (m *GraphQLHandler) RevokeSession(ctx context.Context, input struct {
	Realm string
	ID    int
}) (string, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "AuthDeliveryGraphQL:RevokeSession")
	defer trace.Finish()

	if err := m.uc.Auth().RevokeSession(ctx, input.Realm, input.ID); err != nil {
		return "", err
	}
	return "Success", nil
}

// RevokeUserSessions resolver
func (m *GraphQLHandler) RevokeUserSessions(ctx context.Context, input struct {
	Realm  string
	UserID int
}) (string, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "AuthDeliveryGraphQL:RevokeUserSessions")
	defer trace.Finish()

	if err := m.uc.Auth().RevokeUserSessions(ctx, input.Realm, input.UserID); err != nil {
		return "", err
	}
	return "Success", nil
}
