package domain

import "time"

// Client types
const (
	ClientTypePublic       = "public"
	ClientTypeConfidential = "confidential"
)

// Client model — an application (public) or backend service (confidential) registered in a realm
type Client struct {
	ID            int        `gorm:"column:id;primary_key" json:"id"`
	RealmID       int        `gorm:"column:realm_id" json:"realmId"`
	ClientID      string     `gorm:"column:client_id" json:"clientId"`
	Name          string     `gorm:"column:name" json:"name"`
	Description   string     `gorm:"column:description" json:"description"`
	Type          string     `gorm:"column:type" json:"type"`
	SecretHash    string     `gorm:"column:secret_hash" json:"-"`
	GrantTypes    string     `gorm:"column:grant_types" json:"grantTypes"` // comma separated
	Enabled       bool       `gorm:"column:enabled" json:"enabled"`
	ServiceUserID *int       `gorm:"column:service_user_id" json:"serviceUserId"`
	CreatedAt     time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt     *time.Time `gorm:"column:deleted_at" json:"-"`
}

// TableName return table name of Client model
func (Client) TableName() string { return "clients" }
