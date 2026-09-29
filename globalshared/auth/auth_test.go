package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/golangid/candi/candishared"
	"github.com/stretchr/testify/assert"
)

const issuerBase = "http://user.test"

func newKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

type tokenOpts struct {
	realm, issRealm string
	kid             string
	exp             time.Duration
	sid             int
}

func sign(t *testing.T, key *rsa.PrivateKey, o tokenOpts) string {
	t.Helper()
	if o.issRealm == "" {
		o.issRealm = o.realm
	}
	if o.exp == 0 {
		o.exp = time.Minute
	}
	claim := candishared.TokenClaim{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: IssuerFor(issuerBase, o.issRealm), Subject: "5", ExpiresAt: jwt.NewNumericDate(time.Now().Add(o.exp)),
		},
		Additional: map[string]any{ClaimRealm: o.realm, ClaimSessionID: o.sid, ClaimType: TypeUser},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, &claim)
	tok.Header["kid"] = o.kid
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// jwksServer serves the given realm -> kid -> key sets and counts requests
func jwksServer(t *testing.T, sets map[string]map[string]*rsa.PublicKey) (*httptest.Server, *int32) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		var realm string
		for name := range sets {
			if r.URL.Path == "/v1/realms/"+name+"/.well-known/jwks.json" {
				realm = name
			}
		}
		if realm == "" {
			http.NotFound(w, r)
			return
		}
		var set JWKS
		for kid, pub := range sets[realm] {
			set.Keys = append(set.Keys, NewJWK(pub, kid))
		}
		json.NewEncoder(w).Encode(set)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func Test_TokenValidator(t *testing.T) {
	key := newKey(t)
	srv, _ := jwksServer(t, map[string]map[string]*rsa.PublicKey{"acme": {"k1": &key.PublicKey}})
	v := NewTokenValidator(issuerBase, NewJWKSKeyProvider(srv.URL))
	ctx := context.Background()

	t.Run("valid token", func(t *testing.T) {
		claim, err := v.ValidateToken(ctx, sign(t, key, tokenOpts{realm: "acme", kid: "k1", sid: 3}))
		assert.NoError(t, err)
		assert.Equal(t, "acme", RealmFromClaim(claim))
		assert.Equal(t, 3, SessionIDFromClaim(claim))
		assert.Equal(t, TypeUser, TypeFromClaim(claim))
		assert.Equal(t, "5", claim.Subject)
	})

	t.Run("expired", func(t *testing.T) {
		_, err := v.ValidateToken(ctx, sign(t, key, tokenOpts{realm: "acme", kid: "k1", exp: -time.Minute}))
		assert.Error(t, err)
	})

	t.Run("signed with a different key", func(t *testing.T) {
		_, err := v.ValidateToken(ctx, sign(t, newKey(t), tokenOpts{realm: "acme", kid: "k1"}))
		assert.Error(t, err)
	})

	t.Run("issuer of another deployment", func(t *testing.T) {
		other := NewTokenValidator("http://evil.test", NewJWKSKeyProvider(srv.URL))
		_, err := other.ValidateToken(ctx, sign(t, key, tokenOpts{realm: "acme", kid: "k1"}))
		assert.Error(t, err)
	})

	t.Run("realm claim must match the issuer realm", func(t *testing.T) {
		_, err := v.ValidateToken(ctx, sign(t, key, tokenOpts{realm: "master", issRealm: "acme", kid: "k1"}))
		assert.ErrorContains(t, err, "realm mismatch")
	})

	t.Run("missing kid, unknown kid, garbage", func(t *testing.T) {
		_, err := v.ValidateToken(ctx, sign(t, key, tokenOpts{realm: "acme"}))
		assert.Error(t, err)
		_, err = v.ValidateToken(ctx, sign(t, key, tokenOpts{realm: "acme", kid: "nope"}))
		assert.Error(t, err)
		_, err = v.ValidateToken(ctx, "not-a-jwt")
		assert.Error(t, err)
	})

	t.Run("hs256 / none tokens are refused (algorithm confusion)", func(t *testing.T) {
		hs := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"iss": IssuerFor(issuerBase, "acme"), "exp": time.Now().Add(time.Minute).Unix()})
		hs.Header["kid"] = "k1"
		s, _ := hs.SignedString([]byte("secret"))
		_, err := v.ValidateToken(ctx, s)
		assert.Error(t, err)
		none := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{"iss": IssuerFor(issuerBase, "acme"), "exp": time.Now().Add(time.Minute).Unix()})
		none.Header["kid"] = "k1"
		s, _ = none.SignedString(jwt.UnsafeAllowNoneSignatureType)
		_, err = v.ValidateToken(ctx, s)
		assert.Error(t, err)
	})

	t.Run("tokens of a disabled realm", func(t *testing.T) {
		vv := NewTokenValidator(issuerBase, NewJWKSKeyProvider(srv.URL))
		vv.RealmEnabled = func(context.Context, string) bool { return false }
		_, err := vv.ValidateToken(ctx, sign(t, key, tokenOpts{realm: "acme", kid: "k1"}))
		assert.ErrorContains(t, err, "realm disabled")
	})
}

func Test_JWKSKeyProvider_cachingAndRotation(t *testing.T) {
	k1, k2 := newKey(t), newKey(t)
	sets := map[string]map[string]*rsa.PublicKey{"acme": {"k1": &k1.PublicKey}}
	srv, hits := jwksServer(t, sets)
	p := NewJWKSKeyProvider(srv.URL)
	p.minRefetch = 0
	ctx := context.Background()

	_, err := p.PublicKey(ctx, "acme", "k1")
	assert.NoError(t, err)
	_, err = p.PublicKey(ctx, "acme", "k1")
	assert.NoError(t, err)
	assert.EqualValues(t, 1, atomic.LoadInt32(hits), "second lookup is served from cache")

	// key rotation: an unknown kid triggers a refetch that picks up the new key
	sets["acme"]["k2"] = &k2.PublicKey
	pub, err := p.PublicKey(ctx, "acme", "k2")
	assert.NoError(t, err)
	assert.Equal(t, k2.PublicKey.N, pub.N)
	assert.EqualValues(t, 2, atomic.LoadInt32(hits))

	// a key removed from the JWKS stops verifying after the next refresh
	delete(sets["acme"], "k1")
	_, err = p.PublicKey(ctx, "acme", "k1") // cached: still served
	assert.NoError(t, err)
	_, err = p.PublicKey(ctx, "acme", "unknown") // forces refetch, drops k1
	assert.Error(t, err)
	_, err = p.PublicKey(ctx, "acme", "k1")
	assert.Error(t, err)

	_, err = p.PublicKey(ctx, "ghost", "k1")
	assert.Error(t, err, "unknown realm")
}

func Test_JWKSKeyProvider_rateLimitsRefetch(t *testing.T) {
	srv, hits := jwksServer(t, map[string]map[string]*rsa.PublicKey{"acme": {}})
	p := NewJWKSKeyProvider(srv.URL) // default 10s minimum between refetches
	for i := 0; i < 20; i++ {
		_, _ = p.PublicKey(context.Background(), "acme", "forged-kid")
	}
	assert.EqualValues(t, 1, atomic.LoadInt32(hits), "forged kids must not hammer the user service")
}

func Test_IssuerHelpers(t *testing.T) {
	iss := IssuerFor("http://x/", "acme")
	assert.Equal(t, "http://x/realms/acme", iss)
	realm, ok := RealmFromIssuer("http://x", iss)
	assert.True(t, ok)
	assert.Equal(t, "acme", realm)
	for _, bad := range []string{"http://y/realms/acme", "http://x/realms/", "http://x/realms/a/b", "acme"} {
		_, ok = RealmFromIssuer("http://x", bad)
		assert.False(t, ok, bad)
	}
}

type fakePerm struct {
	calls   int
	allowed bool
	err     error
	got     PermissionRequest
}

func (f *fakePerm) CheckPermission(_ context.Context, r PermissionRequest) (bool, string, error) {
	f.calls++
	f.got = r
	return f.allowed, "admin", f.err
}

func claimCtx(realm string, sid int) context.Context {
	c := &candishared.TokenClaim{RegisteredClaims: jwt.RegisteredClaims{Subject: "5"}}
	c.Additional = map[string]any{ClaimRealm: realm, ClaimSessionID: float64(sid)}
	return context.WithValue(context.Background(), candishared.ContextKeyTokenClaim, c)
}

func Test_ACLChecker(t *testing.T) {
	t.Run("qualifies the code with the service and passes realm and session", func(t *testing.T) {
		f := &fakePerm{allowed: true}
		role, err := NewACLChecker("notification", f, time.Minute).CheckPermission(claimCtx("acme", 3), "5", "sendNotification")
		assert.NoError(t, err)
		assert.Equal(t, "admin", role)
		assert.Equal(t, PermissionRequest{Realm: "acme", UserID: "5", SessionID: 3, Service: "notification", Code: "sendNotification"}, f.got)
	})

	t.Run("denied", func(t *testing.T) {
		_, err := NewACLChecker("s", &fakePerm{}, 0).CheckPermission(claimCtx("acme", 0), "5", "x")
		assert.ErrorContains(t, err, "Forbidden")
	})

	t.Run("checker failure fails closed", func(t *testing.T) {
		_, err := NewACLChecker("s", &fakePerm{allowed: true, err: errors.New("user service down")}, 0).CheckPermission(claimCtx("acme", 0), "5", "x")
		assert.ErrorContains(t, err, "Forbidden")
	})

	t.Run("no claim in context", func(t *testing.T) {
		_, err := NewACLChecker("s", &fakePerm{allowed: true}, 0).CheckPermission(context.Background(), "5", "x")
		assert.Error(t, err)
	})

	t.Run("decisions are cached per realm/user/session/code; failures are not", func(t *testing.T) {
		f := &fakePerm{allowed: true}
		c := NewACLChecker("s", f, time.Minute)
		for i := 0; i < 3; i++ {
			_, _ = c.CheckPermission(claimCtx("acme", 3), "5", "x")
		}
		assert.Equal(t, 1, f.calls)
		_, _ = c.CheckPermission(claimCtx("acme", 4), "5", "x") // another session
		_, _ = c.CheckPermission(claimCtx("globex", 3), "5", "x")
		_, _ = c.CheckPermission(claimCtx("acme", 3), "5", "y")
		assert.Equal(t, 4, f.calls)

		bad := &fakePerm{err: errors.New("down")}
		c = NewACLChecker("s", bad, time.Minute)
		_, _ = c.CheckPermission(claimCtx("acme", 3), "5", "x")
		_, _ = c.CheckPermission(claimCtx("acme", 3), "5", "x")
		assert.Equal(t, 2, bad.calls)
	})

	t.Run("cache expires", func(t *testing.T) {
		f := &fakePerm{allowed: true}
		c := NewACLChecker("s", f, 10*time.Millisecond)
		_, _ = c.CheckPermission(claimCtx("acme", 3), "5", "x")
		time.Sleep(20 * time.Millisecond)
		_, _ = c.CheckPermission(claimCtx("acme", 3), "5", "x")
		assert.Equal(t, 2, f.calls)
	})
}

func Test_ClaimHelpers_nilSafe(t *testing.T) {
	assert.Equal(t, "", RealmFromClaim(nil))
	assert.Equal(t, 0, SessionIDFromClaim(nil))
	assert.Equal(t, "", ClientIDFromClaim(&candishared.TokenClaim{}))
	assert.Equal(t, 7, SessionIDFromClaim(&candishared.TokenClaim{Additional: map[string]any{ClaimSessionID: 7}}))
	assert.Equal(t, 7, SessionIDFromClaim(&candishared.TokenClaim{Additional: map[string]any{ClaimSessionID: float64(7)}}))
}
