package domain

import "time"

// Session model — one refresh token; rotated tokens of the same login share FamilyID
type Session struct {
	ID          int        `gorm:"column:id;primary_key" json:"id"`
	RealmID     int        `gorm:"column:realm_id" json:"realmId"`
	UserID      int        `gorm:"column:user_id" json:"userId"`
	ClientID    int        `gorm:"column:client_id" json:"clientId"`
	FamilyID    int        `gorm:"column:family_id" json:"familyId"`
	RefreshHash string     `gorm:"column:refresh_hash" json:"-"`
	ExpiresAt   time.Time  `gorm:"column:expires_at" json:"expiresAt"`
	RotatedAt   *time.Time `gorm:"column:rotated_at" json:"rotatedAt"`
	RevokedAt   *time.Time `gorm:"column:revoked_at" json:"revokedAt"`
	IP          string     `gorm:"column:ip" json:"ip"`
	UserAgent   string     `gorm:"column:user_agent" json:"userAgent"`
	CreatedAt   time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

// TableName return table name of Session model
func (Session) TableName() string { return "sessions" }
