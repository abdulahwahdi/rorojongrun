package usecase

import (
	"context"
	"crypto/rsa"
	"testing"
	"time"

	"monorepo/services/user/pkg/helper"
	mockauthrepo "monorepo/services/user/pkg/mocks/modules/auth/repository"
	mockclient "monorepo/services/user/pkg/mocks/modules/client/repository"
	mockrbac "monorepo/services/user/pkg/mocks/modules/rbac/repository"
	mockrealm "monorepo/services/user/pkg/mocks/modules/realm/repository"
	mockuserrepo "monorepo/services/user/pkg/mocks/modules/user/repository"
	mocksharedrepo "monorepo/services/user/pkg/mocks/shared/repository"
	"monorepo/services/user/pkg/shared"
	shareddomain "monorepo/services/user/pkg/shared/domain"

	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
)

const testSecret = "unit-test-secret"

// harness wires an auth usecase to mocked repositories
type harness struct {
	uc       *authUsecaseImpl
	repo     *mocksharedrepo.RepoSQL
	realms   *mockrealm.RealmRepository
	keys     *mockrealm.RealmKeyRepository
	users    *mockuserrepo.UserRepository
	clients  *mockclient.ClientRepository
	sessions *mockauthrepo.SessionRepository
	perms    *mockrbac.PermissionRepository
	priv     *rsa.PrivateKey
	kid      string
	realm    shareddomain.Realm
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	shared.SetEnv(shared.Environment{IssuerBaseURL: "http://issuer.test", KeyEncryptionSecret: testSecret})

	h := &harness{
		repo: &mocksharedrepo.RepoSQL{}, realms: &mockrealm.RealmRepository{}, keys: &mockrealm.RealmKeyRepository{},
		users: &mockuserrepo.UserRepository{}, clients: &mockclient.ClientRepository{},
		sessions: &mockauthrepo.SessionRepository{}, perms: &mockrbac.PermissionRepository{},
		kid: "kid-1",
		realm: shareddomain.Realm{
			ID: 1, Name: "acme", Enabled: true, AccessTokenTTLSec: 900, RefreshTokenTTLSec: 3600,
			MaxFailedAttempts: 3, LockoutSec: 60, OTPLoginEnabled: true,
		},
	}
	var err error
	if h.priv, err = helper.GenerateRSAKey(); err != nil {
		t.Fatal(err)
	}
	privPEM, _ := helper.PrivateKeyToPEM(h.priv)
	enc, _ := helper.Encrypt(testSecret, privPEM)

	h.repo.On("RealmRepo").Return(h.realms)
	h.repo.On("RealmKeyRepo").Return(h.keys)
	h.repo.On("UserRepo").Return(h.users)
	h.repo.On("ClientRepo").Return(h.clients)
	h.repo.On("SessionRepo").Return(h.sessions)
	h.repo.On("PermissionRepo").Return(h.perms)
	h.repo.On("WithTransaction", mock.Anything, mock.Anything).Return(func(ctx context.Context, fn func(context.Context) error) error {
		return fn(ctx)
	})
	h.realms.On("FindByName", mock.Anything, "acme").Return(h.realm, nil)
	h.keys.On("FindActive", mock.Anything, 1).Return(shareddomain.RealmKey{ID: 1, RealmID: 1, KID: h.kid, Active: true, PrivateKeyEnc: enc}, nil)
	h.users.On("Roles", mock.Anything, mock.Anything).Return([]shareddomain.Role{{ID: 1, Name: "operator"}}, nil).Maybe()

	h.uc = &authUsecaseImpl{repoSQL: h.repo, keyCache: map[string]*cachedKey{}}
	return h
}

func (h *harness) publicClient(grants string) shareddomain.Client {
	c := shareddomain.Client{ID: 7, RealmID: 1, ClientID: "web", Type: shareddomain.ClientTypePublic, GrantTypes: grants, Enabled: true}
	h.clients.On("FindByClientID", mock.Anything, 1, "web").Return(c, nil)
	return c
}

func (h *harness) user(password string) shareddomain.User {
	hash, _ := helper.HashSecret(password)
	return shareddomain.User{ID: 5, RealmID: 1, Username: "alice", PasswordHash: hash, Status: shareddomain.UserStatusActive}
}

func future() *time.Time { t := time.Now().Add(time.Hour); return &t }
func past() *time.Time   { t := time.Now().Add(-time.Hour); return &t }

var errNotFound = gorm.ErrRecordNotFound

func float2int(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return -1
}
