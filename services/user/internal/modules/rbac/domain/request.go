package domain

// RequestRole is the payload to create / update a role
type RequestRole struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// RequestPermission is the payload to create / update a permission
type RequestPermission struct {
	Service     string `json:"service"`
	Code        string `json:"code"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// RequestPermissionBulk upserts many permissions (identified by service + code) at once
type RequestPermissionBulk struct {
	Permissions []RequestPermission `json:"permissions"`
}

// RequestPermissionIDs is a set of permission ids
type RequestPermissionIDs struct {
	PermissionIDs []int `json:"permissionIds"`
}

// RequestPermissionID is a single permission id
type RequestPermissionID struct {
	PermissionID int `json:"permissionId"`
}
