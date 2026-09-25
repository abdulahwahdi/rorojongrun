package repository

import (
	"context"

	"monorepo/services/user/internal/modules/rbac/domain"
	shareddomain "monorepo/services/user/pkg/shared/domain"
)

// RoleRepository abstract interface
type RoleRepository interface {
	FetchAll(ctx context.Context, realmID int, filter *domain.FilterRole) ([]shareddomain.Role, error)
	Count(ctx context.Context, realmID int, filter *domain.FilterRole) int
	Find(ctx context.Context, realmID, id int) (shareddomain.Role, error)
	FindByName(ctx context.Context, realmID int, name string) (shareddomain.Role, error)
	Save(ctx context.Context, data *shareddomain.Role) error
	// Delete soft deletes the role and drops its user / permission assignments
	Delete(ctx context.Context, id int) error
	CountUsers(ctx context.Context, roleID int) int

	Permissions(ctx context.Context, roleID int) ([]shareddomain.Permission, error)
	ReplacePermissions(ctx context.Context, roleID int, permissionIDs []int) error
	AddPermission(ctx context.Context, roleID, permissionID int) error
	RemovePermission(ctx context.Context, roleID, permissionID int) error
}

// PermissionRepository abstract interface
type PermissionRepository interface {
	FetchAll(ctx context.Context, realmID int, filter *domain.FilterPermission) ([]shareddomain.Permission, error)
	Count(ctx context.Context, realmID int, filter *domain.FilterPermission) int
	Find(ctx context.Context, realmID, id int) (shareddomain.Permission, error)
	FindByServiceCode(ctx context.Context, realmID int, service, code string) (shareddomain.Permission, error)
	// FetchByIDs returns the (not deleted) permissions of the realm with the given ids
	FetchByIDs(ctx context.Context, realmID int, ids []int) ([]shareddomain.Permission, error)
	// CountByIDs returns how many of ids exist (not deleted) in the realm
	CountByIDs(ctx context.Context, realmID int, ids []int) int
	Save(ctx context.Context, data *shareddomain.Permission) error
	// Delete soft deletes the permission, drops it from roles and detaches it from menus
	Delete(ctx context.Context, id int) error

	// EffectiveForUser returns every permission granted to the user through its roles
	EffectiveForUser(ctx context.Context, realmID, userID int) ([]shareddomain.Permission, error)
}
