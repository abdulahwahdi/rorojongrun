package domain

import "time"

// Permission types
const (
	PermissionTypeAPI = "api"
	PermissionTypeUI  = "ui"
)

// WildcardPermission matches every service / every code
const WildcardPermission = "*"

// Role model
type Role struct {
	ID          int        `gorm:"column:id;primary_key" json:"id"`
	RealmID     int        `gorm:"column:realm_id" json:"realmId"`
	Name        string     `gorm:"column:name" json:"name"`
	Description string     `gorm:"column:description" json:"description"`
	CreatedAt   time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt   *time.Time `gorm:"column:deleted_at" json:"-"`
}

// TableName return table name of Role model
func (Role) TableName() string { return "roles" }

// Permission model, identified by (realm, service, code)
type Permission struct {
	ID          int        `gorm:"column:id;primary_key" json:"id"`
	RealmID     int        `gorm:"column:realm_id" json:"realmId"`
	Service     string     `gorm:"column:service" json:"service"`
	Code        string     `gorm:"column:code" json:"code"`
	Type        string     `gorm:"column:type" json:"type"`
	Description string     `gorm:"column:description" json:"description"`
	CreatedAt   time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt   *time.Time `gorm:"column:deleted_at" json:"-"`
}

// TableName return table name of Permission model
func (Permission) TableName() string { return "permissions" }

// RolePermission model
type RolePermission struct {
	RoleID       int `gorm:"column:role_id;primary_key"`
	PermissionID int `gorm:"column:permission_id;primary_key"`
}

// TableName return table name of RolePermission model
func (RolePermission) TableName() string { return "role_permissions" }
