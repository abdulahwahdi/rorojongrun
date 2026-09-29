package graphqlhandler

import (
	"context"

	"monorepo/services/user/internal/modules/user/domain"

	"github.com/golangid/candi/tracer"
)

const ok = "Success"

// CreateUser resolver
func (m *GraphQLHandler) CreateUser(ctx context.Context, input struct {
	Realm string
	Data  domain.RequestCreateUser
}) (domain.ResponseUser, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "UserDeliveryGraphQL:CreateUser")
	defer trace.Finish()

	return m.uc.User().CreateUser(ctx, input.Realm, &input.Data)
}

// UpdateUser resolver
func (m *GraphQLHandler) UpdateUser(ctx context.Context, input struct {
	Realm string
	ID    int
	Data  domain.RequestUpdateUser
}) (string, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "UserDeliveryGraphQL:UpdateUser")
	defer trace.Finish()

	if err := m.uc.User().UpdateUser(ctx, input.Realm, input.ID, &input.Data); err != nil {
		return "", err
	}
	return ok, nil
}

// DeleteUser resolver
func (m *GraphQLHandler) DeleteUser(ctx context.Context, input struct {
	Realm string
	ID    int
}) (string, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "UserDeliveryGraphQL:DeleteUser")
	defer trace.Finish()

	if err := m.uc.User().DeleteUser(ctx, input.Realm, input.ID); err != nil {
		return "", err
	}
	return ok, nil
}

// SetUserPassword resolver
func (m *GraphQLHandler) SetUserPassword(ctx context.Context, input struct {
	Realm    string
	ID       int
	Password string
}) (string, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "UserDeliveryGraphQL:SetUserPassword")
	defer trace.Finish()

	if err := m.uc.User().SetPassword(ctx, input.Realm, input.ID, input.Password); err != nil {
		return "", err
	}
	return ok, nil
}

// UnlockUser resolver
func (m *GraphQLHandler) UnlockUser(ctx context.Context, input struct {
	Realm string
	ID    int
}) (string, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "UserDeliveryGraphQL:UnlockUser")
	defer trace.Finish()

	if err := m.uc.User().UnlockUser(ctx, input.Realm, input.ID); err != nil {
		return "", err
	}
	return ok, nil
}

// AddUserRole resolver
func (m *GraphQLHandler) AddUserRole(ctx context.Context, input struct {
	Realm  string
	UserID int
	RoleID int
}) (string, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "UserDeliveryGraphQL:AddUserRole")
	defer trace.Finish()

	if err := m.uc.User().AddUserRole(ctx, input.Realm, input.UserID, input.RoleID); err != nil {
		return "", err
	}
	return ok, nil
}

// ReplaceUserRoles resolver
func (m *GraphQLHandler) ReplaceUserRoles(ctx context.Context, input struct {
	Realm   string
	UserID  int
	RoleIDs []int
}) (string, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "UserDeliveryGraphQL:ReplaceUserRoles")
	defer trace.Finish()

	if err := m.uc.User().ReplaceUserRoles(ctx, input.Realm, input.UserID, input.RoleIDs); err != nil {
		return "", err
	}
	return ok, nil
}

// RemoveUserRole resolver
func (m *GraphQLHandler) RemoveUserRole(ctx context.Context, input struct {
	Realm  string
	UserID int
	RoleID int
}) (string, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "UserDeliveryGraphQL:RemoveUserRole")
	defer trace.Finish()

	if err := m.uc.User().RemoveUserRole(ctx, input.Realm, input.UserID, input.RoleID); err != nil {
		return "", err
	}
	return ok, nil
}
