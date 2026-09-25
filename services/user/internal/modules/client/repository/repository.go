package repository

import (
	"context"

	"monorepo/services/user/internal/modules/client/domain"
	shareddomain "monorepo/services/user/pkg/shared/domain"
)

// ClientRepository abstract interface
type ClientRepository interface {
	FetchAll(ctx context.Context, realmID int, filter *domain.FilterClient) ([]shareddomain.Client, error)
	Count(ctx context.Context, realmID int, filter *domain.FilterClient) int
	Find(ctx context.Context, realmID, id int) (shareddomain.Client, error)
	FindByClientID(ctx context.Context, realmID int, clientID string) (shareddomain.Client, error)
	Save(ctx context.Context, data *shareddomain.Client) error
	Delete(ctx context.Context, id int) error
}
