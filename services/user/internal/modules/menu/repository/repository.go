package repository

import (
	"context"

	"monorepo/services/user/internal/modules/menu/domain"
	shareddomain "monorepo/services/user/pkg/shared/domain"
)

// MenuRepository abstract interface
type MenuRepository interface {
	FetchAll(ctx context.Context, clientID int, filter *domain.FilterMenu) ([]shareddomain.Menu, error)
	Count(ctx context.Context, clientID int, filter *domain.FilterMenu) int
	// FetchAllOfClient returns every menu of the client ordered by sort order (for tree building)
	FetchAllOfClient(ctx context.Context, clientID int) ([]shareddomain.Menu, error)
	Find(ctx context.Context, clientID, id int) (shareddomain.Menu, error)
	FindByKey(ctx context.Context, clientID int, key string) (shareddomain.Menu, error)
	Save(ctx context.Context, data *shareddomain.Menu) error
	DeleteMany(ctx context.Context, ids []int) error
	DeleteByClient(ctx context.Context, clientID int) error
}
