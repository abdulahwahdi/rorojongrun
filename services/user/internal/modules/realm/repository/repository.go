package repository

import (
	"context"

	"monorepo/services/user/internal/modules/realm/domain"
	shareddomain "monorepo/services/user/pkg/shared/domain"
)

// RealmRepository abstract interface
type RealmRepository interface {
	FetchAll(ctx context.Context, filter *domain.FilterRealm) ([]shareddomain.Realm, error)
	Count(ctx context.Context, filter *domain.FilterRealm) int
	FindByID(ctx context.Context, id int) (shareddomain.Realm, error)
	FindByName(ctx context.Context, name string) (shareddomain.Realm, error)
	Save(ctx context.Context, data *shareddomain.Realm) error
	Delete(ctx context.Context, id int) error
}

// RealmKeyRepository abstract interface
type RealmKeyRepository interface {
	FetchAll(ctx context.Context, realmID int, filter *domain.FilterRealmKey) ([]shareddomain.RealmKey, error)
	Count(ctx context.Context, realmID int, filter *domain.FilterRealmKey) int
	Find(ctx context.Context, realmID, id int) (shareddomain.RealmKey, error)
	// FindByKID looks the key up across realms
	FindByKID(ctx context.Context, kid string) (shareddomain.RealmKey, error)
	// FindActive returns the newest active key of the realm
	FindActive(ctx context.Context, realmID int) (shareddomain.RealmKey, error)
	// FetchActive returns all active keys of the realm (published in the JWKS)
	FetchActive(ctx context.Context, realmID int) ([]shareddomain.RealmKey, error)
	Save(ctx context.Context, data *shareddomain.RealmKey) error
	Delete(ctx context.Context, realmID, id int) error
}
