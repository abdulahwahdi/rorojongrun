package domain

// RequestRealm is the payload to create / update a realm. Pointer fields are optional:
// on create they fall back to defaults, on update they leave the stored value untouched.
type RequestRealm struct {
	Name               string  `json:"name"` // immutable, ignored on update
	DisplayName        *string `json:"displayName"`
	Enabled            *bool   `json:"enabled"`
	AccessTokenTTLSec  *int    `json:"accessTokenTtlSec"`
	RefreshTokenTTLSec *int    `json:"refreshTokenTtlSec"`
	MaxFailedAttempts  *int    `json:"maxFailedAttempts"`
	LockoutSec         *int    `json:"lockoutSec"`
	OTPLoginEnabled    *bool   `json:"otpLoginEnabled"`
}

// RequestRealmKey is the payload to (de)activate a signing key
type RequestRealmKey struct {
	Active bool `json:"active"`
}
