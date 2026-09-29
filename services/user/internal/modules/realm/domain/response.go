package domain

import (
	"time"

	shareddomain "monorepo/services/user/pkg/shared/domain"

	"github.com/golangid/candi/candishared"
)

// ResponseRealmList model
type ResponseRealmList struct {
	Meta candishared.Meta `json:"meta"`
	Data []ResponseRealm  `json:"data"`
}

// ResponseRealm model
type ResponseRealm struct {
	ID                 int    `json:"id"`
	Name               string `json:"name"`
	DisplayName        string `json:"displayName"`
	Enabled            bool   `json:"enabled"`
	AccessTokenTTLSec  int    `json:"accessTokenTtlSec"`
	RefreshTokenTTLSec int    `json:"refreshTokenTtlSec"`
	MaxFailedAttempts  int    `json:"maxFailedAttempts"`
	LockoutSec         int    `json:"lockoutSec"`
	OTPLoginEnabled    bool   `json:"otpLoginEnabled"`
	CreatedAt          string `json:"createdAt"`
	UpdatedAt          string `json:"updatedAt"`
}

// Serialize from db model
func (r *ResponseRealm) Serialize(s *shareddomain.Realm) {
	r.ID, r.Name, r.DisplayName, r.Enabled = s.ID, s.Name, s.DisplayName, s.Enabled
	r.AccessTokenTTLSec, r.RefreshTokenTTLSec = s.AccessTokenTTLSec, s.RefreshTokenTTLSec
	r.MaxFailedAttempts, r.LockoutSec, r.OTPLoginEnabled = s.MaxFailedAttempts, s.LockoutSec, s.OTPLoginEnabled
	r.CreatedAt, r.UpdatedAt = s.CreatedAt.Format(time.RFC3339), s.UpdatedAt.Format(time.RFC3339)
}

// ResponseRealmKeyList model
type ResponseRealmKeyList struct {
	Meta candishared.Meta   `json:"meta"`
	Data []ResponseRealmKey `json:"data"`
}

// ResponseRealmKey model — only the public half of the key is ever exposed
type ResponseRealmKey struct {
	ID           int    `json:"id"`
	KID          string `json:"kid"`
	Algorithm    string `json:"algorithm"`
	PublicKeyPEM string `json:"publicKeyPem"`
	Active       bool   `json:"active"`
	CreatedAt    string `json:"createdAt"`
}

// Serialize from db model
func (r *ResponseRealmKey) Serialize(s *shareddomain.RealmKey) {
	r.ID, r.KID, r.Algorithm, r.PublicKeyPEM, r.Active = s.ID, s.KID, s.Algorithm, s.PublicKeyPEM, s.Active
	r.CreatedAt = s.CreatedAt.Format(time.RFC3339)
}

// ResponseOpenIDConfiguration is the minimal OIDC discovery document of a realm
type ResponseOpenIDConfiguration struct {
	Issuer               string   `json:"issuer"`
	JWKSURI              string   `json:"jwks_uri"`
	TokenEndpoint        string   `json:"token_endpoint"`
	GrantTypesSupported  []string `json:"grant_types_supported"`
	SigningAlgsSupported []string `json:"id_token_signing_alg_values_supported"`
}
