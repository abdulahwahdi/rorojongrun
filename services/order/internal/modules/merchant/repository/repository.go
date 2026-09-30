package repository

import (
	"context"

	"monorepo/services/order/internal/modules/merchant/domain"
	shareddomain "monorepo/services/order/pkg/shared/domain"
)

// MerchantRepository abstract interface
type MerchantRepository interface {
	Find(ctx context.Context, merchantID string) (shareddomain.MerchantSetting, error)
	FetchAll(ctx context.Context, filter *domain.FilterMerchant) ([]shareddomain.MerchantSetting, error)
	Count(ctx context.Context, filter *domain.FilterMerchant) int
	// Save inserts or replaces a merchant's settings
	Save(ctx context.Context, data *shareddomain.MerchantSetting) error
	Delete(ctx context.Context, merchantID string) (deleted bool, err error)
}
