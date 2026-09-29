package repository

import (
	"context"

	shareddomain "monorepo/services/payment/pkg/shared/domain"
)

// GatewayRepository abstract interface. Gateways are a small fixed set (one row per
// provider), so there is no paging.
type GatewayRepository interface {
	FetchAll(ctx context.Context) ([]shareddomain.Gateway, error)
	FindByCode(ctx context.Context, code string) (shareddomain.Gateway, error)
	Save(ctx context.Context, data *shareddomain.Gateway) error
}
