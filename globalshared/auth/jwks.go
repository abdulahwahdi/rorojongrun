package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

// JWK is a single RSA public key in JSON Web Key format
type JWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// JWKS is a JSON Web Key Set
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// NewJWK builds the JWK of an RSA public key
func NewJWK(pub *rsa.PublicKey, kid string) JWK {
	return JWK{
		Kty: "RSA", Kid: kid, Use: "sig", Alg: "RS256",
		N: base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		E: base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}
}

// PublicKey decodes the JWK into an RSA public key
func (k JWK) PublicKey() (*rsa.PublicKey, error) {
	if k.Kty != "RSA" {
		return nil, fmt.Errorf("unsupported key type %q", k.Kty)
	}
	n, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, err
	}
	e, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, err
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}, nil
}

// KeyProvider resolves the public key that verifies tokens of a realm
type KeyProvider interface {
	PublicKey(ctx context.Context, realm, kid string) (*rsa.PublicKey, error)
}

// JWKSKeyProvider fetches realm JWKS from the user service and caches them.
// An unknown kid triggers a refetch (key rotation), rate limited per realm so a
// forged kid cannot be used to hammer the user service.
type JWKSKeyProvider struct {
	userServiceURL string
	client         *http.Client
	minRefetch     time.Duration

	mu        sync.RWMutex
	keys      map[string]*rsa.PublicKey // realm + "/" + kid
	lastFetch map[string]time.Time      // realm
}

// NewJWKSKeyProvider constructor. userServiceURL is the HTTP base url of the user service.
func NewJWKSKeyProvider(userServiceURL string) *JWKSKeyProvider {
	return &JWKSKeyProvider{
		userServiceURL: strings.TrimRight(userServiceURL, "/"),
		client:         &http.Client{Timeout: 5 * time.Second},
		minRefetch:     10 * time.Second,
		keys:           map[string]*rsa.PublicKey{},
		lastFetch:      map[string]time.Time{},
	}
}

// PublicKey implement KeyProvider
func (p *JWKSKeyProvider) PublicKey(ctx context.Context, realm, kid string) (*rsa.PublicKey, error) {
	cacheKey := realm + "/" + kid
	p.mu.RLock()
	key, ok := p.keys[cacheKey]
	last := p.lastFetch[realm]
	p.mu.RUnlock()
	if ok {
		return key, nil
	}
	if time.Since(last) < p.minRefetch {
		return nil, errors.New("unknown signing key")
	}
	if err := p.refresh(ctx, realm); err != nil {
		return nil, err
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if key, ok = p.keys[cacheKey]; ok {
		return key, nil
	}
	return nil, errors.New("unknown signing key")
}

func (p *JWKSKeyProvider) refresh(ctx context.Context, realm string) error {
	p.mu.Lock()
	p.lastFetch[realm] = time.Now()
	p.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.userServiceURL+"/v1/realms/"+realm+"/.well-known/jwks.json", nil)
	if err != nil {
		return err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch jwks: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch jwks: status %d", resp.StatusCode)
	}
	var set JWKS
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		return fmt.Errorf("decode jwks: %w", err)
	}

	fresh := map[string]*rsa.PublicKey{}
	for _, k := range set.Keys {
		if pub, err := k.PublicKey(); err == nil {
			fresh[realm+"/"+k.Kid] = pub
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for ck := range p.keys { // drop keys of this realm that were removed
		if strings.HasPrefix(ck, realm+"/") {
			delete(p.keys, ck)
		}
	}
	for ck, pub := range fresh {
		p.keys[ck] = pub
	}
	return nil
}
