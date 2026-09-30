package usecase

import (
	"context"
	"time"

	"monorepo/services/order/internal/modules/merchant/domain"
	shareddomain "monorepo/services/order/pkg/shared/domain"
	"monorepo/services/order/pkg/shared/repository"
	"monorepo/services/order/pkg/shared/usecase/common"

	"github.com/golangid/candi/codebase/factory/dependency"
)

// MerchantUsecase abstraction: per-merchant numbering, tax, rounding and invoice seller data
type MerchantUsecase interface {
	GetAllMerchants(ctx context.Context, filter *domain.FilterMerchant) (res domain.ResponseMerchantList, err error)
	// GetMerchant returns a merchant's own settings (404 when it has none, see MerchantSettings)
	GetMerchant(ctx context.Context, merchantID string) (res shareddomain.MerchantSetting, err error)
	SaveMerchant(ctx context.Context, merchantID string, req *domain.RequestSaveMerchant) (res shareddomain.MerchantSetting, err error)
	DeleteMerchant(ctx context.Context, merchantID string) (err error)
	// Quote prices a basket with a merchant's settings, the same breakdown the ledger records
	Quote(ctx context.Context, merchantID string, lines []shareddomain.PriceLine) (res shareddomain.Breakdown, err error)

	// shared with the other modules (common.Usecase)
	MerchantSettings(ctx context.Context, merchantID string) (shareddomain.MerchantSetting, error)
	NextOrderNumber(ctx context.Context, m shareddomain.MerchantSetting, at time.Time) (string, error)
	NextInvoiceNumber(ctx context.Context, m shareddomain.MerchantSetting, invoiceType string, at time.Time) (string, error)
}

type merchantUsecaseImpl struct {
	deps          dependency.Dependency
	sharedUsecase common.Usecase
	repoSQL       repository.RepoSQL
}

// NewMerchantUsecase usecase impl constructor
func NewMerchantUsecase(deps dependency.Dependency) (MerchantUsecase, func(sharedUsecase common.Usecase)) {
	uc := &merchantUsecaseImpl{deps: deps, repoSQL: repository.GetSharedRepoSQL()}
	return uc, func(sharedUsecase common.Usecase) {
		uc.sharedUsecase = sharedUsecase
	}
}
