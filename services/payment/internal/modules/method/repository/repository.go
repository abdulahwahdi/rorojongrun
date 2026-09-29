package repository

import (
	"context"

	"monorepo/services/payment/internal/modules/method/domain"
	shareddomain "monorepo/services/payment/pkg/shared/domain"
)

// MethodRepository abstract interface
type MethodRepository interface {
	FetchAll(ctx context.Context, filter *domain.FilterMethod) ([]shareddomain.Method, error)
	Count(ctx context.Context, filter *domain.FilterMethod) int
	FindByID(ctx context.Context, id int) (shareddomain.Method, error)
	FindByCode(ctx context.Context, code string) (shareddomain.Method, error)
	// FetchEnabled returns every enabled method in checkout display order
	FetchEnabled(ctx context.Context) ([]shareddomain.Method, error)
	Save(ctx context.Context, data *shareddomain.Method) error
	Delete(ctx context.Context, id int) error
}
