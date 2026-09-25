package usecase

import (
	"context"
	"testing"

	"monorepo/globalshared/auth"
	"monorepo/services/user/internal/modules/realm/domain"
	"monorepo/services/user/pkg/helper"
	mockauthrepo "monorepo/services/user/pkg/mocks/modules/auth/repository"
	mockrbac "monorepo/services/user/pkg/mocks/modules/rbac/repository"
	mockrealm "monorepo/services/user/pkg/mocks/modules/realm/repository"
	mocksharedrepo "monorepo/services/user/pkg/mocks/shared/repository"
	"monorepo/services/user/pkg/shared"
	shareddomain "monorepo/services/user/pkg/shared/domain"
	"monorepo/services/user/pkg/shared/usecase/common"

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
	uc       *realmUsecaseImpl
	realms   *mockrealm.RealmRepository
	keys     *mockrealm.RealmKeyRepository
	roles    *mockrbac.RoleRepository
	perms    *mockrbac.PermissionRepository
	sessions *mockauthrepo.SessionRepository
}

func newHarness(t *testing.T) *harness {
	shared.SetEnv(shared.Environment{IssuerBaseURL: "http://issuer.test", KeyEncryptionSecret: "unit-secret"})
	repo := &mocksharedrepo.RepoSQL{}
	h := &harness{
		realms: &mockrealm.RealmRepository{}, keys: &mockrealm.RealmKeyRepository{}, roles: &mockrbac.RoleRepository{},
		perms: &mockrbac.PermissionRepository{}, sessions: &mockauthrepo.SessionRepository{},
	}
	repo.On("RealmRepo").Return(h.realms)
	repo.On("RealmKeyRepo").Return(h.keys)
	repo.On("RoleRepo").Return(h.roles)
	repo.On("PermissionRepo").Return(h.perms)
	repo.On("SessionRepo").Return(h.sessions)
	repo.On("WithTransaction", mock.Anything, mock.Anything).Return(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) })
	h.uc = &realmUsecaseImpl{repoSQL: repo}
	return h
}

func Test_CreateRealm(t *testing.T) {
	t.Run("creates the realm with a signing key and a realm-admin role", func(t *testing.T) {
		h := newHarness(t)
		h.realms.On("FindByName", mock.Anything, "acme").Return(shareddomain.Realm{}, errNF)
		h.realms.On("Save", mock.Anything, mock.MatchedBy(func(r *shareddomain.Realm) bool {
			return r.Name == "acme" && r.Enabled && r.AccessTokenTTLSec == 900 && r.MaxFailedAttempts == 5
		})).Run(func(args mock.Arguments) { args.Get(1).(*shareddomain.Realm).ID = 4 }).Return(nil)
		h.keys.On("Save", mock.Anything, mock.MatchedBy(func(k *shareddomain.RealmKey) bool {
			// the private half must never be stored in clear
			return k.RealmID == 4 && k.Active && k.KID != "" && k.PrivateKeyEnc != "" &&
				!assert.ObjectsAreEqual(k.PrivateKeyEnc[:5], "-----") && k.PublicKeyPEM != ""
		})).Return(nil)
		h.perms.On("Save", mock.Anything, mock.MatchedBy(func(p *shareddomain.Permission) bool {
			return p.RealmID == 4 && p.Service == "user" && p.Code == "*"
		})).Run(func(args mock.Arguments) { args.Get(1).(*shareddomain.Permission).ID = 8 }).Return(nil)
		h.roles.On("Save", mock.Anything, mock.MatchedBy(func(r *shareddomain.Role) bool { return r.RealmID == 4 && r.Name == "realm-admin" })).
			Run(func(args mock.Arguments) { args.Get(1).(*shareddomain.Role).ID = 9 }).Return(nil)
		h.roles.On("ReplacePermissions", mock.Anything, 9, []int{8}).Return(nil)

		res, err := h.uc.CreateRealm(ctxOf(common.MasterRealm), &domain.RequestRealm{Name: "acme"})
		assert.NoError(t, err)
		assert.Equal(t, "acme", res.Name)
		h.keys.AssertNumberOfCalls(t, "Save", 1)
		h.roles.AssertCalled(t, "ReplacePermissions", mock.Anything, 9, []int{8})
	})

	t.Run("only the master realm may create realms", func(t *testing.T) {
		h := newHarness(t)
		_, err := h.uc.CreateRealm(ctxOf("acme"), &domain.RequestRealm{Name: "other"})
		assert.Equal(t, 403, helper.HTTPStatus(err))
		_, err = h.uc.GetAllRealm(ctxOf("acme"), &domain.FilterRealm{})
		assert.Equal(t, 403, helper.HTTPStatus(err))
	})

	t.Run("name must be a slug and unique", func(t *testing.T) {
		h := newHarness(t)
		for _, bad := range []string{"", "Acme", "a b", "-a", "a/b", "../x"} {
			_, err := h.uc.CreateRealm(ctxOf(common.MasterRealm), &domain.RequestRealm{Name: bad})
			assert.Equal(t, 400, helper.HTTPStatus(err), bad)
		}
		h.realms.On("FindByName", mock.Anything, "acme").Return(shareddomain.Realm{ID: 1}, nil)
		_, err := h.uc.CreateRealm(ctxOf(common.MasterRealm), &domain.RequestRealm{Name: "acme"})
		assert.Equal(t, 409, helper.HTTPStatus(err))
	})

	t.Run("token ttl sanity", func(t *testing.T) {
		h := newHarness(t)
		h.realms.On("FindByName", mock.Anything, "acme").Return(shareddomain.Realm{}, errNF)
		access, refresh := 600, 60
		_, err := h.uc.CreateRealm(ctxOf(common.MasterRealm), &domain.RequestRealm{Name: "acme", AccessTokenTTLSec: &access, RefreshTokenTTLSec: &refresh})
		assert.Equal(t, 400, helper.HTTPStatus(err))
	})
}

func Test_UpdateAndDeleteRealm(t *testing.T) {
	acme := shareddomain.Realm{ID: 4, Name: "acme", Enabled: true, AccessTokenTTLSec: 900, RefreshTokenTTLSec: 3600}

	t.Run("the master realm can be neither disabled nor deleted", func(t *testing.T) {
		h := newHarness(t)
		h.realms.On("FindByName", mock.Anything, "master").Return(shareddomain.Realm{ID: 1, Name: "master", Enabled: true, AccessTokenTTLSec: 900, RefreshTokenTTLSec: 3600}, nil)
		off := false
		assert.Equal(t, 400, helper.HTTPStatus(h.uc.UpdateRealm(ctxOf(common.MasterRealm), "master", &domain.RequestRealm{Enabled: &off})))
		assert.Equal(t, 400, helper.HTTPStatus(h.uc.DeleteRealm(ctxOf(common.MasterRealm), "master")))
	})

	t.Run("update keeps unspecified fields and ignores a rename", func(t *testing.T) {
		h := newHarness(t)
		h.realms.On("FindByName", mock.Anything, "acme").Return(acme, nil)
		h.realms.On("Save", mock.Anything, mock.MatchedBy(func(r *shareddomain.Realm) bool {
			return r.Name == "acme" && r.DisplayName == "ACME" && r.AccessTokenTTLSec == 900 && r.OTPLoginEnabled
		})).Return(nil)
		name, on := "ACME", true
		assert.NoError(t, h.uc.UpdateRealm(ctxOf("acme"), "acme", &domain.RequestRealm{Name: "renamed", DisplayName: &name, OTPLoginEnabled: &on}))
	})

	t.Run("a realm cannot be managed by the token of another realm", func(t *testing.T) {
		h := newHarness(t)
		assert.Equal(t, 403, helper.HTTPStatus(h.uc.UpdateRealm(ctxOf("globex"), "acme", &domain.RequestRealm{})))
	})

	t.Run("delete revokes sessions then soft deletes; only master may", func(t *testing.T) {
		h := newHarness(t)
		h.realms.On("FindByName", mock.Anything, "acme").Return(acme, nil)
		h.sessions.On("RevokeByRealm", mock.Anything, 4).Return(nil)
		h.realms.On("Delete", mock.Anything, 4).Return(nil)
		assert.Equal(t, 403, helper.HTTPStatus(h.uc.DeleteRealm(ctxOf("acme"), "acme")))
		assert.NoError(t, h.uc.DeleteRealm(ctxOf(common.MasterRealm), "acme"))
		h.sessions.AssertCalled(t, "RevokeByRealm", mock.Anything, 4)
	})
}

func Test_Keys(t *testing.T) {
	acme := shareddomain.Realm{ID: 4, Name: "acme", Enabled: true}
	setup := func() *harness {
		h := newHarness(t)
		h.realms.On("FindByName", mock.Anything, "acme").Return(acme, nil)
		return h
	}

	t.Run("the last active key cannot be deactivated, an active key cannot be deleted", func(t *testing.T) {
		h := setup()
		h.keys.On("Find", mock.Anything, 4, 1).Return(shareddomain.RealmKey{ID: 1, RealmID: 4, Active: true}, nil)
		h.keys.On("FetchActive", mock.Anything, 4).Return([]shareddomain.RealmKey{{ID: 1}}, nil)
		assert.Equal(t, 409, helper.HTTPStatus(h.uc.UpdateRealmKey(ctxOf("acme"), "acme", 1, &domain.RequestRealmKey{Active: false})))
		assert.Equal(t, 409, helper.HTTPStatus(h.uc.DeleteRealmKey(ctxOf("acme"), "acme", 1)))
		h.keys.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
	})

	t.Run("with another active key the old one can be retired and deleted", func(t *testing.T) {
		h := setup()
		h.keys.On("Find", mock.Anything, 4, 1).Return(shareddomain.RealmKey{ID: 1, RealmID: 4, Active: true}, nil).Once()
		h.keys.On("FetchActive", mock.Anything, 4).Return([]shareddomain.RealmKey{{ID: 1}, {ID: 2}}, nil)
		h.keys.On("Save", mock.Anything, mock.MatchedBy(func(k *shareddomain.RealmKey) bool { return !k.Active })).Return(nil)
		assert.NoError(t, h.uc.UpdateRealmKey(ctxOf("acme"), "acme", 1, &domain.RequestRealmKey{Active: false}))

		h.keys.On("Find", mock.Anything, 4, 1).Return(shareddomain.RealmKey{ID: 1, RealmID: 4, Active: false}, nil)
		h.keys.On("Delete", mock.Anything, 4, 1).Return(nil)
		assert.NoError(t, h.uc.DeleteRealmKey(ctxOf("acme"), "acme", 1))
	})

	t.Run("responses never carry the private key", func(t *testing.T) {
		h := setup()
		h.keys.On("Find", mock.Anything, 4, 1).Return(shareddomain.RealmKey{ID: 1, KID: "k", PublicKeyPEM: "PUB", PrivateKeyEnc: "SECRET"}, nil)
		res, err := h.uc.GetDetailRealmKey(ctxOf("acme"), "acme", 1)
		assert.NoError(t, err)
		assert.Equal(t, "PUB", res.PublicKeyPEM)
		assert.NotContains(t, res.KID+res.PublicKeyPEM+res.Algorithm, "SECRET")
	})

	t.Run("jwks is public and publishes only active keys as valid RSA JWKs", func(t *testing.T) {
		h := setup()
		priv, _ := helper.GenerateRSAKey()
		pem, _ := helper.PublicKeyToPEM(&priv.PublicKey)
		h.keys.On("FetchActive", mock.Anything, 4).Return([]shareddomain.RealmKey{{KID: "k1", PublicKeyPEM: string(pem)}}, nil)
		set, err := h.uc.GetJWKS(context.Background(), "acme") // no claim in ctx: public
		assert.NoError(t, err)
		assert.Len(t, set.Keys, 1)
		back, err := set.Keys[0].PublicKey()
		assert.NoError(t, err)
		assert.Equal(t, priv.PublicKey.N, back.N)
		assert.Equal(t, "k1", set.Keys[0].Kid)
	})

	t.Run("openid configuration", func(t *testing.T) {
		h := setup()
		cfg, err := h.uc.GetOpenIDConfiguration(context.Background(), "acme")
		assert.NoError(t, err)
		assert.Equal(t, auth.IssuerFor("http://issuer.test", "acme"), cfg.Issuer)
		assert.Equal(t, "http://issuer.test/v1/realms/acme/.well-known/jwks.json", cfg.JWKSURI)
	})
}
