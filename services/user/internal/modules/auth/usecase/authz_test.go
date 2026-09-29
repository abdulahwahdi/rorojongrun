package usecase

import (
	"context"
	"testing"

	"monorepo/globalshared/auth"
	"monorepo/services/user/internal/modules/auth/domain"
	menudomain "monorepo/services/user/internal/modules/menu/domain"
	mockcommon "monorepo/services/user/pkg/mocks/shared/usecase/common"
	shareddomain "monorepo/services/user/pkg/shared/domain"
	"monorepo/services/user/pkg/shared/usecase/common"

	"github.com/golang-jwt/jwt/v5"
	"github.com/golangid/candi/candishared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func perm(service, code string) shareddomain.Permission {
	return shareddomain.Permission{ID: 1, RealmID: 1, Service: service, Code: code}
}

func Test_CheckPermission(t *testing.T) {
	check := func(h *harness, sid int, service, code string) (bool, string, error) {
		return h.uc.CheckPermission(context.Background(), domain.CheckPermissionRequest{Realm: "acme", UserID: "5", SessionID: sid, Service: service, Code: code})
	}
	setup := func(h *harness, perms ...shareddomain.Permission) {
		h.users.On("Find", mock.Anything, 1, 5).Return(h.user("x"), nil)
		h.perms.On("EffectiveForUser", mock.Anything, 1, 5).Return(perms, nil)
	}

	t.Run("exact grant", func(t *testing.T) {
		h := newHarness(t)
		setup(h, perm("notification", "sendNotification"))
		ok, role, err := check(h, 0, "notification", "sendNotification")
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, "operator", role)
	})

	t.Run("a grant for another service or code does not leak", func(t *testing.T) {
		h := newHarness(t)
		setup(h, perm("notification", "sendNotification"))
		ok, _, _ := check(h, 0, "order", "sendNotification")
		assert.False(t, ok)
		ok, _, _ = check(h, 0, "notification", "deleteTemplate")
		assert.False(t, ok)
	})

	t.Run("wildcards", func(t *testing.T) {
		h := newHarness(t)
		setup(h, perm("*", "*"))
		ok, _, _ := check(h, 0, "anything", "at-all")
		assert.True(t, ok)

		h = newHarness(t)
		setup(h, perm("user", "*"))
		ok, _, _ = check(h, 0, "user", "createUser")
		assert.True(t, ok)
		ok, _, _ = check(h, 0, "order", "createUser")
		assert.False(t, ok)
	})

	t.Run("revoked session is denied even with the permission", func(t *testing.T) {
		h := newHarness(t)
		setup(h, perm("*", "*"))
		h.sessions.On("IsFamilyActive", mock.Anything, 3).Return(false, nil)
		ok, _, err := check(h, 3, "user", "x")
		assert.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("live session", func(t *testing.T) {
		h := newHarness(t)
		setup(h, perm("*", "*"))
		h.sessions.On("IsFamilyActive", mock.Anything, 3).Return(true, nil)
		ok, _, _ := check(h, 3, "user", "x")
		assert.True(t, ok)
	})

	t.Run("disabled user, unknown user, unknown / disabled realm, bad subject", func(t *testing.T) {
		h := newHarness(t)
		u := h.user("x")
		u.Status = shareddomain.UserStatusDisabled
		h.users.On("Find", mock.Anything, 1, 5).Return(u, nil)
		ok, _, _ := check(h, 0, "user", "x")
		assert.False(t, ok)

		h = newHarness(t)
		h.users.On("Find", mock.Anything, 1, 5).Return(shareddomain.User{}, errNotFound)
		ok, _, _ = check(h, 0, "user", "x")
		assert.False(t, ok)

		h = newHarness(t)
		ok, _, _ = h.uc.CheckPermission(context.Background(), domain.CheckPermissionRequest{Realm: "acme", UserID: "abc", Service: "a", Code: "b"})
		assert.False(t, ok)

		h = newHarness(t)
		h.realms.ExpectedCalls = nil
		h.realms.On("FindByName", mock.Anything, "acme").Return(shareddomain.Realm{}, errNotFound)
		ok, _, err := check(h, 0, "user", "x")
		assert.False(t, ok)
		assert.NoError(t, err)

		h = newHarness(t)
		h.realms.ExpectedCalls = nil
		r := h.realm
		r.Enabled = false
		h.realms.On("FindByName", mock.Anything, "acme").Return(r, nil)
		ok, _, _ = check(h, 0, "user", "x")
		assert.False(t, ok)
	})
}

func claimCtx(realm string, sub string, sid int) context.Context {
	claim := &candishared.TokenClaim{RegisteredClaims: jwt.RegisteredClaims{Subject: sub}}
	claim.Additional = map[string]any{auth.ClaimRealm: realm, auth.ClaimSessionID: float64(sid), auth.ClaimType: auth.TypeUser}
	return candishared.SetToContext(context.Background(), candishared.ContextKeyTokenClaim, claim)
}

func Test_GetMyPermissions(t *testing.T) {
	t.Run("returns codes and the menu filtered by the shared usecase", func(t *testing.T) {
		h := newHarness(t)
		h.users.On("Find", mock.Anything, 1, 5).Return(h.user("x"), nil)
		h.sessions.On("IsFamilyActive", mock.Anything, 3).Return(true, nil)
		perms := []shareddomain.Permission{perm("ui", "menu.orders"), perm("notification", "getAllTemplates")}
		h.perms.On("EffectiveForUser", mock.Anything, 1, 5).Return(perms, nil)
		h.clients.On("FindByClientID", mock.Anything, 1, "web").Return(shareddomain.Client{ID: 7, ClientID: "web"}, nil)
		shared := &mockcommon.Usecase{}
		shared.On("GetUserMenu", mock.Anything, 1, 7, mock.Anything).Return([]menudomain.ResponseMenu{{Key: "orders"}}, nil)
		h.uc.sharedUsecase = shared

		res, err := h.uc.GetMyPermissions(claimCtx("acme", "5", 3), "acme", "web")
		assert.NoError(t, err)
		assert.Equal(t, []domain.ResponsePermissionRef{
			{Service: "notification", Code: "getAllTemplates"}, {Service: "ui", Code: "menu.orders"},
		}, res.Permissions)
		assert.Equal(t, "orders", res.Menus[0].Key)
		assert.Equal(t, []string{"operator"}, res.Roles)
	})

	t.Run("a token of another realm is refused", func(t *testing.T) {
		h := newHarness(t)
		_, err := h.uc.GetMyPermissions(claimCtx("other", "5", 3), "acme", "")
		assert.Equal(t, 403, statusOf(err))
	})

	t.Run("a revoked session is refused", func(t *testing.T) {
		h := newHarness(t)
		h.users.On("Find", mock.Anything, 1, 5).Return(h.user("x"), nil)
		h.sessions.On("IsFamilyActive", mock.Anything, 3).Return(false, nil)
		_, err := h.uc.GetMe(claimCtx("acme", "5", 3), "acme")
		assert.Equal(t, 401, statusOf(err))
	})

	t.Run("no token", func(t *testing.T) {
		h := newHarness(t)
		_, err := h.uc.GetMe(context.Background(), "acme")
		assert.Equal(t, 401, statusOf(err))
	})
}

func Test_Logout(t *testing.T) {
	h := newHarness(t)
	h.sessions.On("RevokeFamily", mock.Anything, 3).Return(nil)
	assert.NoError(t, h.uc.Logout(claimCtx("acme", "5", 3), "acme"))
	h.sessions.AssertCalled(t, "RevokeFamily", mock.Anything, 3)

	h = newHarness(t)
	assert.NoError(t, h.uc.Logout(claimCtx("acme", "5", 0), "acme")) // service token: nothing to revoke
	h.sessions.AssertNotCalled(t, "RevokeFamily", mock.Anything, mock.Anything)
	assert.Equal(t, 403, statusOf(h.uc.Logout(claimCtx("other", "5", 3), "acme")))
}

func Test_RealmAuthorization(t *testing.T) {
	// admin session endpoints go through common.ResolveRealm: same realm or master only
	newSystem := func(realm string) context.Context { return claimCtx(realm, "1", 0) }
	h := newHarness(t)
	h.sessions.On("Find", mock.Anything, 1, 4).Return(shareddomain.Session{ID: 4, FamilyID: 4}, nil)
	h.sessions.On("RevokeFamily", mock.Anything, 4).Return(nil)

	assert.NoError(t, h.uc.RevokeSession(newSystem("acme"), "acme", 4))
	assert.NoError(t, h.uc.RevokeSession(newSystem(common.MasterRealm), "acme", 4))
	assert.Equal(t, 403, statusOf(h.uc.RevokeSession(newSystem("globex"), "acme", 4)))
	assert.Equal(t, 403, statusOf(h.uc.RevokeSession(context.Background(), "acme", 4)))
}

func Test_RevokeMySession_onlyOwn(t *testing.T) {
	h := newHarness(t)
	h.users.On("Find", mock.Anything, 1, 5).Return(h.user("x"), nil)
	h.sessions.On("IsFamilyActive", mock.Anything, 3).Return(true, nil)
	h.sessions.On("Find", mock.Anything, 1, 4).Return(shareddomain.Session{ID: 4, UserID: 99, FamilyID: 4}, nil)
	assert.Equal(t, 404, statusOf(h.uc.RevokeMySession(claimCtx("acme", "5", 3), "acme", 4)))
	h.sessions.AssertNotCalled(t, "RevokeFamily", mock.Anything, mock.Anything)
}
