package repository

import (
	"context"

	"monorepo/services/order/internal/modules/shift/domain"
	shareddomain "monorepo/services/order/pkg/shared/domain"
)

// ShiftRepository abstract interface
type ShiftRepository interface {
	Create(ctx context.Context, data *shareddomain.CashShift) error
	Update(ctx context.Context, data *shareddomain.CashShift) error
	FindByID(ctx context.Context, id int64) (shareddomain.CashShift, error)
	// LockByID reads a shift with SELECT ... FOR UPDATE; call inside WithTransaction
	LockByID(ctx context.Context, id int64) (shareddomain.CashShift, error)
	// FindOpen finds the open shift of a cashier at an outlet
	FindOpen(ctx context.Context, merchantID, outletID, cashierID string) (shareddomain.CashShift, error)
	FetchAll(ctx context.Context, filter *domain.FilterShift) ([]shareddomain.CashShift, error)
	Count(ctx context.Context, filter *domain.FilterShift) int
}
