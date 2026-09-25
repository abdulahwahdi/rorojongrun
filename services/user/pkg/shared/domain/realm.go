package domain

import "time"

// Realm model — an isolated namespace of users, roles, clients, menus and signing keys
type Realm struct {
	ID                 int        `gorm:"column:id;primary_key" json:"id"`
	Name               string     `gorm:"column:name" json:"name"`
	DisplayName        string     `gorm:"column:display_name" json:"displayName"`
	Enabled            bool       `gorm:"column:enabled" json:"enabled"`
	AccessTokenTTLSec  int        `gorm:"column:access_token_ttl_sec" json:"accessTokenTtlSec"`
	RefreshTokenTTLSec int        `gorm:"column:refresh_token_ttl_sec" json:"refreshTokenTtlSec"`
	MaxFailedAttempts  int        `gorm:"column:max_failed_attempts" json:"maxFailedAttempts"`
	LockoutSec         int        `gorm:"column:lockout_sec" json:"lockoutSec"`
	OTPLoginEnabled    bool       `gorm:"column:otp_login_enabled" json:"otpLoginEnabled"`
	CreatedAt          time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt          time.Time  `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt          *time.Time `gorm:"column:deleted_at" json:"-"`
}

// TableName return table name of Realm model
func (Realm) TableName() string { return "realms" }

// RealmKey model — RSA signing key of a realm, private half is AES-GCM encrypted
type RealmKey struct {
	ID            int       `gorm:"column:id;primary_key" json:"id"`
	RealmID       int       `gorm:"column:realm_id" json:"realmId"`
	KID           string    `gorm:"column:kid" json:"kid"`
	Algorithm     string    `gorm:"column:algorithm" json:"algorithm"`
	PublicKeyPEM  string    `gorm:"column:public_key_pem" json:"publicKeyPem"`
	PrivateKeyEnc string    `gorm:"column:private_key_enc" json:"-"`
	Active        bool      `gorm:"column:active" json:"active"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName return table name of RealmKey model
func (RealmKey) TableName() string { return "realm_keys" }
