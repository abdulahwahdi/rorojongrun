package domain

import "time"

// Menu model — one node of the UI menu tree of a client app
type Menu struct {
	ID           int        `gorm:"column:id;primary_key" json:"id"`
	RealmID      int        `gorm:"column:realm_id" json:"realmId"`
	ClientID     int        `gorm:"column:client_id" json:"clientId"`
	ParentID     *int       `gorm:"column:parent_id" json:"parentId"`
	Key          string     `gorm:"column:key" json:"key"`
	Label        string     `gorm:"column:label" json:"label"`
	Path         string     `gorm:"column:path" json:"path"`
	Icon         string     `gorm:"column:icon" json:"icon"`
	SortOrder    int        `gorm:"column:sort_order" json:"sortOrder"`
	PermissionID *int       `gorm:"column:permission_id" json:"permissionId"`
	CreatedAt    time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt    *time.Time `gorm:"column:deleted_at" json:"-"`
}

// TableName return table name of Menu model
func (Menu) TableName() string { return "menus" }
