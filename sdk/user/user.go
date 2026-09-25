package user

import "context"

// User client abstract interface — the internal authorization API of the user service
type User interface {
	// CheckPermission tells whether a principal of a realm may use a permission code of a service.
	CheckPermission(ctx context.Context, req CheckPermissionRequest) (CheckPermissionResponse, error)
	// GetUser fetches a user of a realm
	GetUser(ctx context.Context, realm string, id int) (UserResponse, error)
	// GetUserPermissions lists the effective permission codes of a user for a service
	GetUserPermissions(ctx context.Context, realm string, userID int, service string) ([]string, error)
}

// CheckPermissionRequest is the payload for CheckPermission. Kept as a local copy of the
// service's shape — sdk/ clients must not import a service's internal/ packages.
type CheckPermissionRequest struct {
	Realm     string
	UserID    string
	SessionID int // 0 for tokens without a session (service tokens)
	Service   string
	Code      string
}

// CheckPermissionResponse is the result of CheckPermission
type CheckPermissionResponse struct {
	Allowed bool
	Role    string // comma separated role names
}

// UserResponse mirrors the user model of the user service
type UserResponse struct {
	ID       int
	Realm    string
	Username string
	Email    string
	Phone    string
	FullName string
	Status   string
	Roles    []string
}
