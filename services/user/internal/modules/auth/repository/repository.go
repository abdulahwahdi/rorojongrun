package repository

import (
	"context"

	"monorepo/services/user/internal/modules/auth/domain"
	shareddomain "monorepo/services/user/pkg/shared/domain"
)

// SessionRepository abstract interface
type SessionRepository interface {
	FetchAll(ctx context.Context, realmID int, filter *domain.FilterSession) ([]shareddomain.Session, error)
	Count(ctx context.Context, realmID int, filter *domain.FilterSession) int
	Find(ctx context.Context, realmID, id int) (shareddomain.Session, error)
	FindByRefreshHash(ctx context.Context, hash string) (shareddomain.Session, error)
	// Save inserts a session; when FamilyID is 0 the session starts a new family (family id = its own id)
	Save(ctx context.Context, data *shareddomain.Session) error
	// MarkRotated atomically flags a refresh token as used; false means it was already used
	MarkRotated(ctx context.Context, id int) (bool, error)
	// IsFamilyActive is true when the family exists and none of its sessions is revoked
	IsFamilyActive(ctx context.Context, familyID int) (bool, error)

	RevokeFamily(ctx context.Context, familyID int) error
	RevokeByUser(ctx context.Context, userID int) error
	RevokeByClient(ctx context.Context, clientID int) error
	RevokeByRealm(ctx context.Context, realmID int) error
}
