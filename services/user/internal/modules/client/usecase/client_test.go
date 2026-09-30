package usecase

import (
	"context"
	"testing"

	"monorepo/globalshared/rest"
	"monorepo/services/user/internal/modules/client/domain"
	"monorepo/services/user/pkg/helper"
	mockauthrepo "monorepo/services/user/pkg/mocks/modules/auth/repository"
	mockclient "monorepo/services/user/pkg/mocks/modules/client/repository"
	mockmenu "monorepo/services/user/pkg/mocks/modules/menu/repository"
	mockrealm "monorepo/services/user/pkg/mocks/modules/realm/repository"
	mockuser "monorepo/services/user/pkg/mocks/modules/user/repository"
	mocksharedrepo "monorepo/services/user/pkg/mocks/shared/repository"
	shareddomain "monorepo/services/user/pkg/shared/domain"

	"github.com/golang-jwt/jwt/v5"
	"github.com/golangid/candi/candishared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
)

var errNF = gorm.ErrRecordNotFound

func ctxOf(realm string) context.Context {
	claim := &candishared.TokenClaim{RegisteredClaims: jwt.RegisteredClaims{Subject: "1"}}
	claim.Additional = map[string]any{"realm": realm}
	return candishared.SetToContext(context.Background(), candishared.ContextKeyTokenClaim, claim)
}

type harness struct {
	uc       *clientUsecaseImpl
	clients  *mockclient.ClientRepository
	users    *mockuser.UserRepository
	sessions *mockauthrepo.SessionRepository
	menus    *mockmenu.MenuRepository
}

func newHarness(t *testing.T) *harness {
	repo := &mocksharedrepo.RepoSQL{}
	realms := &mockrealm.RealmRepository{}
	h := &harness{clients: &mockclient.ClientRepository{}, users: &mockuser.UserRepository{}, sessions: &mockauthrepo.SessionRepository{}, menus: &mockmenu.MenuRepository{}}
	repo.On("RealmRepo").Return(realms)
	repo.On("ClientRepo").Return(h.clients)
	repo.On("UserRepo").Return(h.users)
	repo.On("SessionRepo").Return(h.sessions)
	repo.On("MenuRepo").Return(h.menus)
	repo.On("WithTransaction", mock.Anything, mock.Anything).Return(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) })
	realms.On("FindByName", mock.Anything, "acme").Return(shareddomain.Realm{ID: 1, Name: "acme", Enabled: true}, nil)
	h.uc = &clientUsecaseImpl{repoSQL: repo}
	return h
}

func Test_normalizeGrants(t *testing.T) {
	g, err := normalizeGrants("public", nil)
	assert.NoError(t, err)
	assert.Equal(t, "password,refresh_token", g)
	g, err = normalizeGrants("confidential", nil)
	assert.NoError(t, err)
	assert.Equal(t, "client_credentials", g)
	g, err = normalizeGrants("public", []string{"otp", "password", "otp"})
	assert.NoError(t, err)
	assert.Equal(t, "otp,password", g)
	_, err = normalizeGrants("public", []string{"client_credentials"})
	assert.Equal(t, 400, rest.HTTPStatus(err), "machine grant needs a secret to protect it")
	_, err = normalizeGrants("public", []string{"implicit"})
	assert.Equal(t, 400, rest.HTTPStatus(err))
}

func Test_CreateClient(t *testing.T) {
	t.Run("public client has no secret and no service account", func(t *testing.T) {
		h := newHarness(t)
		h.clients.On("FindByClientID", mock.Anything, 1, "web").Return(shareddomain.Client{}, errNF)
		h.clients.On("Save", mock.Anything, mock.MatchedBy(func(c *shareddomain.Client) bool {
			return c.SecretHash == "" && c.ServiceUserID == nil && c.Enabled && c.Type == "public"
		})).Return(nil)
		res, err := h.uc.CreateClient(ctxOf("acme"), "acme", &domain.RequestCreateClient{ClientID: "web"})
		assert.NoError(t, err)
		assert.Empty(t, res.ClientSecret)
		h.users.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
	})

	t.Run("confidential client gets a one-time secret (stored hashed) and a service account", func(t *testing.T) {
		h := newHarness(t)
		h.clients.On("FindByClientID", mock.Anything, 1, "billing").Return(shareddomain.Client{}, errNF)
		h.users.On("Save", mock.Anything, mock.MatchedBy(func(u *shareddomain.User) bool {
			return u.IsServiceAccount && u.Username == "svc-billing" && u.PasswordHash == ""
		})).Run(func(args mock.Arguments) { args.Get(1).(*shareddomain.User).ID = 33 }).Return(nil)
		var stored shareddomain.Client
		h.clients.On("Save", mock.Anything, mock.Anything).Run(func(args mock.Arguments) { stored = *args.Get(1).(*shareddomain.Client) }).Return(nil)

		res, err := h.uc.CreateClient(ctxOf("acme"), "acme", &domain.RequestCreateClient{ClientID: "billing", Type: "confidential"})
		assert.NoError(t, err)
		assert.NotEmpty(t, res.ClientSecret)
		assert.NotEqual(t, res.ClientSecret, stored.SecretHash)
		assert.True(t, helper.CheckSecret(stored.SecretHash, res.ClientSecret))
		assert.Equal(t, 33, *stored.ServiceUserID)
		assert.Equal(t, 33, res.ServiceUserID)
	})

	t.Run("duplicate client id, bad type", func(t *testing.T) {
		h := newHarness(t)
		h.clients.On("FindByClientID", mock.Anything, 1, "web").Return(shareddomain.Client{ID: 1}, nil)
		_, err := h.uc.CreateClient(ctxOf("acme"), "acme", &domain.RequestCreateClient{ClientID: "web"})
		assert.Equal(t, 409, rest.HTTPStatus(err))
		_, err = h.uc.CreateClient(ctxOf("acme"), "acme", &domain.RequestCreateClient{ClientID: "x", Type: "weird"})
		assert.Equal(t, 400, rest.HTTPStatus(err))
	})
}

func Test_UpdateClient(t *testing.T) {
	existing := shareddomain.Client{ID: 7, RealmID: 1, ClientID: "web", Type: "public", GrantTypes: "password", Enabled: true}

	t.Run("disabling revokes the client's sessions; omitted grants are kept", func(t *testing.T) {
		h := newHarness(t)
		h.clients.On("Find", mock.Anything, 1, 7).Return(existing, nil)
		h.clients.On("Save", mock.Anything, mock.MatchedBy(func(c *shareddomain.Client) bool {
			return !c.Enabled && c.GrantTypes == "password" && c.ClientID == "web"
		})).Return(nil)
		h.sessions.On("RevokeByClient", mock.Anything, 7).Return(nil)
		off := false
		assert.NoError(t, h.uc.UpdateClient(ctxOf("acme"), "acme", 7, &domain.RequestUpdateClient{Name: "Web", Enabled: &off}))
		h.sessions.AssertCalled(t, "RevokeByClient", mock.Anything, 7)
	})

	t.Run("a public client cannot be given client_credentials", func(t *testing.T) {
		h := newHarness(t)
		h.clients.On("Find", mock.Anything, 1, 7).Return(existing, nil)
		err := h.uc.UpdateClient(ctxOf("acme"), "acme", 7, &domain.RequestUpdateClient{GrantTypes: []string{"client_credentials"}})
		assert.Equal(t, 400, rest.HTTPStatus(err))
	})
}

func Test_DeleteAndRotate(t *testing.T) {
	svc := 33
	confidential := shareddomain.Client{ID: 8, RealmID: 1, ClientID: "billing", Type: "confidential", ServiceUserID: &svc}

	t.Run("delete cleans sessions, menus and the service account", func(t *testing.T) {
		h := newHarness(t)
		h.clients.On("Find", mock.Anything, 1, 8).Return(confidential, nil)
		h.sessions.On("RevokeByClient", mock.Anything, 8).Return(nil)
		h.menus.On("DeleteByClient", mock.Anything, 8).Return(nil)
		h.users.On("Delete", mock.Anything, 33).Return(nil)
		h.clients.On("Delete", mock.Anything, 8).Return(nil)
		assert.NoError(t, h.uc.DeleteClient(ctxOf("acme"), "acme", 8))
		h.users.AssertCalled(t, "Delete", mock.Anything, 33)
		h.menus.AssertCalled(t, "DeleteByClient", mock.Anything, 8)
	})

	t.Run("rotating replaces the hash and returns the new secret once", func(t *testing.T) {
		h := newHarness(t)
		old, _ := helper.HashSecret("old")
		c := confidential
		c.SecretHash = old
		h.clients.On("Find", mock.Anything, 1, 8).Return(c, nil)
		var stored shareddomain.Client
		h.clients.On("Save", mock.Anything, mock.Anything).Run(func(args mock.Arguments) { stored = *args.Get(1).(*shareddomain.Client) }).Return(nil)
		res, err := h.uc.RotateClientSecret(ctxOf("acme"), "acme", 8)
		assert.NoError(t, err)
		assert.False(t, helper.CheckSecret(stored.SecretHash, "old"))
		assert.True(t, helper.CheckSecret(stored.SecretHash, res.ClientSecret))
	})

	t.Run("public clients have no secret to rotate", func(t *testing.T) {
		h := newHarness(t)
		h.clients.On("Find", mock.Anything, 1, 7).Return(shareddomain.Client{ID: 7, Type: "public"}, nil)
		_, err := h.uc.RotateClientSecret(ctxOf("acme"), "acme", 7)
		assert.Equal(t, 400, rest.HTTPStatus(err))
	})
}
