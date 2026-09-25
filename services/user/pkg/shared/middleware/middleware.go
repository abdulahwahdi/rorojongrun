// Package middleware wires this service's own authentication and authorization. Other services
// verify tokens through JWKS and ask this service over gRPC; here the keys and permissions are
// local, so the same decision logic (AuthUsecase.CheckPermission) is called in-process.
package middleware

import (
	"context"
	"crypto/rsa"
	"errors"
	"sync"
	"time"

	"monorepo/globalshared/auth"
	"monorepo/services/user/pkg/helper"
	"monorepo/services/user/pkg/shared"
	"monorepo/services/user/pkg/shared/repository"
	"monorepo/services/user/pkg/shared/usecase"
	"monorepo/services/user/pkg/shared/usecase/common"

	"monorepo/services/user/internal/modules/auth/domain"
)

// aclCacheTTL is how long a permission decision is reused; it bounds how long a revoked
// session or removed role keeps working on this service
const aclCacheTTL = 5 * time.Second

// NewTokenValidator validates tokens of every realm against the signing keys stored in the database
func NewTokenValidator() *auth.TokenValidator {
	keys := &localKeyProvider{ttl: 30 * time.Second, keys: map[string]cachedKey{}}
	v := auth.NewTokenValidator(shared.GetEnv().IssuerBaseURL, keys)
	v.RealmEnabled = keys.realmEnabled
	return v
}

// NewACLChecker checks permissions of the "user" service itself
func NewACLChecker() *auth.ACLChecker {
	return auth.NewACLChecker(common.AdminService, localPermissionClient{}, aclCacheTTL)
}

type localPermissionClient struct{}

func (localPermissionClient) CheckPermission(ctx context.Context, req auth.PermissionRequest) (bool, string, error) {
	return usecase.GetSharedUsecase().Auth().CheckPermission(ctx, domain.CheckPermissionRequest{
		Realm: req.Realm, UserID: req.UserID, SessionID: req.SessionID, Service: req.Service, Code: req.Code,
	})
}

type cachedKey struct {
	pub     *rsa.PublicKey
	expires time.Time
}

type localKeyProvider struct {
	ttl time.Duration

	mu      sync.Mutex
	keys    map[string]cachedKey // realm/kid
	enabled map[string]cachedEnabled
}

type cachedEnabled struct {
	ok      bool
	expires time.Time
}

// PublicKey implement auth.KeyProvider. Only active keys of the realm the token claims to be from verify.
func (p *localKeyProvider) PublicKey(ctx context.Context, realm, kid string) (*rsa.PublicKey, error) {
	ck := realm + "/" + kid
	p.mu.Lock()
	c, ok := p.keys[ck]
	p.mu.Unlock()
	if ok && time.Now().Before(c.expires) {
		return c.pub, nil
	}

	repo := repository.GetSharedRepoSQL()
	r, err := repo.RealmRepo().FindByName(ctx, realm)
	if err != nil {
		return nil, errors.New("unknown realm")
	}
	key, err := repo.RealmKeyRepo().FindByKID(ctx, kid)
	if err != nil || key.RealmID != r.ID || !key.Active {
		return nil, errors.New("unknown signing key")
	}
	pub, err := helper.PublicKeyFromPEM([]byte(key.PublicKeyPEM))
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	p.keys[ck] = cachedKey{pub: pub, expires: time.Now().Add(p.ttl)}
	p.mu.Unlock()
	return pub, nil
}

func (p *localKeyProvider) realmEnabled(ctx context.Context, realm string) bool {
	p.mu.Lock()
	if p.enabled == nil {
		p.enabled = map[string]cachedEnabled{}
	}
	c, ok := p.enabled[realm]
	p.mu.Unlock()
	if ok && time.Now().Before(c.expires) {
		return c.ok
	}
	r, err := repository.GetSharedRepoSQL().RealmRepo().FindByName(ctx, realm)
	enabled := err == nil && r.Enabled
	p.mu.Lock()
	p.enabled[realm] = cachedEnabled{ok: enabled, expires: time.Now().Add(p.ttl)}
	p.mu.Unlock()
	return enabled
}
