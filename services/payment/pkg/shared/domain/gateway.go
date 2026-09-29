package domain

import "time"

// Gateway model — one row per payment gateway (midtrans, xendit, mock)
type Gateway struct {
	ID             int       `gorm:"column:id;primary_key" json:"id"`
	Code           string    `gorm:"column:code;type:varchar(50)" json:"code"`
	Name           string    `gorm:"column:name;type:varchar(100)" json:"name"`
	IsEnabled      bool      `gorm:"column:is_enabled" json:"isEnabled"`
	Environment    string    `gorm:"column:environment;type:varchar(20)" json:"environment"`
	Settings       JSON      `gorm:"column:settings;type:jsonb" json:"settings"`
	CredentialsEnc string    `gorm:"column:credentials_enc" json:"-"`
	CreatedAt      time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt      time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName return table name of Gateway model
func (Gateway) TableName() string { return "payment_gateways" }
