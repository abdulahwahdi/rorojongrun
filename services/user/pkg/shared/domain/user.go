package domain

import "time"

// User statuses
const (
	UserStatusActive   = "active"
	UserStatusDisabled = "disabled"
	UserStatusLocked   = "locked"
)

// User model, scoped to a realm
type User struct {
	ID               int        `gorm:"column:id;primary_key" json:"id"`
	RealmID          int        `gorm:"column:realm_id" json:"realmId"`
	Username         string     `gorm:"column:username" json:"username"`
	Email            *string    `gorm:"column:email" json:"email"`
	Phone            *string    `gorm:"column:phone" json:"phone"`
	FullName         string     `gorm:"column:full_name" json:"fullName"`
	PasswordHash     string     `gorm:"column:password_hash" json:"-"`
	Status           string     `gorm:"column:status" json:"status"`
	FailedAttempts   int        `gorm:"column:failed_attempts" json:"failedAttempts"`
	LockedUntil      *time.Time `gorm:"column:locked_until" json:"lockedUntil"`
	LastLoginAt      *time.Time `gorm:"column:last_login_at" json:"lastLoginAt"`
	IsServiceAccount bool       `gorm:"column:is_service_account" json:"isServiceAccount"`
	CreatedAt        time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt        *time.Time `gorm:"column:deleted_at" json:"-"`
}

// TableName return table name of User model
func (User) TableName() string { return "users" }

// UserRole model
type UserRole struct {
	UserID int `gorm:"column:user_id;primary_key"`
	RoleID int `gorm:"column:role_id;primary_key"`
}

// TableName return table name of UserRole model
func (UserRole) TableName() string { return "user_roles" }
