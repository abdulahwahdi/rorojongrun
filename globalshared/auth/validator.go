package auth

import (
	"context"
	"errors"

	"github.com/golang-jwt/jwt/v5"
	"github.com/golangid/candi/candishared"
)

// TokenValidator verifies RS256 tokens issued by the user service.
// Implements candi's interfaces.TokenValidator.
type TokenValidator struct {
	issuerBaseURL string
	keys          KeyProvider
	// RealmEnabled is optional; when set, tokens of realms it rejects are refused
	RealmEnabled func(ctx context.Context, realm string) bool
}

// NewTokenValidator constructor
func NewTokenValidator(issuerBaseURL string, keys KeyProvider) *TokenValidator {
	return &TokenValidator{issuerBaseURL: issuerBaseURL, keys: keys}
}

// ValidateToken implement interfaces.TokenValidator
func (v *TokenValidator) ValidateToken(ctx context.Context, token string) (*candishared.TokenClaim, error) {
	var claim candishared.TokenClaim
	_, err := jwt.ParseWithClaims(token, &claim, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("missing kid")
		}
		iss, err := t.Claims.GetIssuer()
		if err != nil {
			return nil, err
		}
		realm, ok := RealmFromIssuer(v.issuerBaseURL, iss)
		if !ok {
			return nil, errors.New("invalid issuer")
		}
		return v.keys.PublicKey(ctx, realm, kid)
	}, jwt.WithValidMethods([]string{"RS256"}), jwt.WithExpirationRequired())
	if err != nil {
		return nil, errors.New("Invalid token: " + err.Error())
	}

	realm, _ := RealmFromIssuer(v.issuerBaseURL, claim.Issuer)
	if RealmFromClaim(&claim) != realm {
		return nil, errors.New("Invalid token: realm mismatch")
	}
	if v.RealmEnabled != nil && !v.RealmEnabled(ctx, realm) {
		return nil, errors.New("Invalid token: realm disabled")
	}
	return &claim, nil
}
