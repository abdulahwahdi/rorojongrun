package usecase

import (
	"context"
	"time"

	"monorepo/services/order/internal/modules/shift/domain"
	shareddomain "monorepo/services/order/pkg/shared/domain"
	"monorepo/services/order/pkg/shared/repository"
	"monorepo/services/order/pkg/shared/usecase/common"

	"github.com/golangid/candi/codebase/factory/dependency"
)

// ShiftUsecase abstraction: cashier shifts and cash reconciliation
type ShiftUsecase interface {
	OpenShift(ctx context.Context, actor string, req *domain.RequestOpenShift) (res shareddomain.CashShift, err error)
	CloseShift(ctx context.Context, id int64, actor string, req *domain.RequestCloseShift) (res domain.ResponseShift, err error)
	// CurrentShift is the open shift of a cashier at an outlet
	CurrentShift(ctx context.Context, merchantID, outletID, cashierID string) (res domain.ResponseShift, err error)
	GetShift(ctx context.Context, id int64) (res domain.ResponseShift, err error)
	GetAllShifts(ctx context.Context, filter *domain.FilterShift) (res domain.ResponseShiftList, err error)

	// shared with the other modules (common.Usecase)
	AttachCashSale(ctx context.Context, order *shareddomain.Order) error
	AttachCashRefund(ctx context.Context, order *shareddomain.Order, cashierID string) error
}

type shiftUsecaseImpl struct {
	deps          dependency.Dependency
	sharedUsecase common.Usecase
	repoSQL       repository.RepoSQL
	now           func() time.Time
}

// NewShiftUsecase usecase impl constructor
func NewShiftUsecase(deps dependency.Dependency) (ShiftUsecase, func(sharedUsecase common.Usecase)) {
	uc := &shiftUsecaseImpl{deps: deps, repoSQL: repository.GetSharedRepoSQL(), now: time.Now}
	return uc, func(sharedUsecase common.Usecase) {
		uc.sharedUsecase = sharedUsecase
	}
}
