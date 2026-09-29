package domain

import "time"

// OTPRequest model — a single OTP verification lifecycle
type OTPRequest struct {
	ID           int        `gorm:"column:id;primary_key" json:"id"`
	Recipient    string     `gorm:"column:recipient;type:varchar(255)" json:"recipient"`
	Channel      string     `gorm:"column:channel;type:varchar(20)" json:"channel"`
	Purpose      string     `gorm:"column:purpose;type:varchar(50)" json:"purpose"`
	CodeHash     string     `gorm:"column:code_hash" json:"-"`
	ExpiresAt    time.Time  `gorm:"column:expires_at" json:"expiresAt"`
	MaxAttempts  int        `gorm:"column:max_attempts" json:"maxAttempts"`
	AttemptCount int        `gorm:"column:attempt_count" json:"attemptCount"`
	VerifiedAt   *time.Time `gorm:"column:verified_at" json:"verifiedAt,omitempty"`
	CreatedAt    time.Time  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt    time.Time  `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName return table name of OTPRequest model
func (OTPRequest) TableName() string { return "otp_requests" }
