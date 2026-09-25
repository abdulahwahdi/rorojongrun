package repository

import (
	"context"

	"monorepo/services/user/internal/modules/user/domain"
	shareddomain "monorepo/services/user/pkg/shared/domain"
)

// UserRepository abstract interface
type UserRepository interface {
	FetchAll(ctx context.Context, realmID int, filter *domain.FilterUser) ([]shareddomain.User, error)
	Count(ctx context.Context, realmID int, filter *domain.FilterUser) int
	Find(ctx context.Context, realmID, id int) (shareddomain.User, error)
	FindByUsername(ctx context.Context, realmID int, username string) (shareddomain.User, error)
	FindByEmail(ctx context.Context, realmID int, email string) (shareddomain.User, error)
	FindByPhone(ctx context.Context, realmID int, phone string) (shareddomain.User, error)
	Save(ctx context.Context, data *shareddomain.User) error
	// Delete soft deletes the user and drops its role assignments
	Delete(ctx context.Context, id int) error

	Roles(ctx context.Context, userID int) ([]shareddomain.Role, error)
	ReplaceRoles(ctx context.Context, userID int, roleIDs []int) error
	AddRole(ctx context.Context, userID, roleID int) error
	RemoveRole(ctx context.Context, userID, roleID int) error
	// CountRolesByIDs returns how many of ids exist (not deleted) in the realm
	CountRolesByIDs(ctx context.Context, realmID int, ids []int) int
}
