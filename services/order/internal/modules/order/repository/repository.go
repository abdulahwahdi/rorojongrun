package repository

import (
	"context"

	"monorepo/services/order/internal/modules/order/domain"
	shareddomain "monorepo/services/order/pkg/shared/domain"
)

// OrderRepository abstract interface. It covers the order aggregate: orders, their items and timeline.
type OrderRepository interface {
	// LockPayment serialises everything that happens to the order of a payment until the
	// surrounding transaction ends (a transaction-scoped advisory lock); call inside WithTransaction
	LockPayment(ctx context.Context, paymentID string) error
	FindByPaymentID(ctx context.Context, paymentID string) (shareddomain.Order, error)
	FindByID(ctx context.Context, id int64) (shareddomain.Order, error)
	FindByNumber(ctx context.Context, number string) (shareddomain.Order, error)
	// LockByID reads an order with SELECT ... FOR UPDATE; call inside WithTransaction
	LockByID(ctx context.Context, id int64) (shareddomain.Order, error)
	// Create inserts the order and its items
	Create(ctx context.Context, data *shareddomain.Order) error
	// Update writes every column of the order but its keys and items
	Update(ctx context.Context, data *shareddomain.Order) error
	FetchItems(ctx context.Context, orderIDs ...int64) ([]shareddomain.OrderItem, error)

	// EventExists reports whether a payment event was already recorded (call under LockPayment)
	EventExists(ctx context.Context, orderID int64, event, transactionID string) (bool, error)
	// InsertEvent appends to the timeline; a payment event already recorded returns inserted=false
	InsertEvent(ctx context.Context, data *shareddomain.OrderEvent) (inserted bool, err error)
	FetchEvents(ctx context.Context, orderID int64) ([]shareddomain.OrderEvent, error)

	FetchAll(ctx context.Context, filter *shareddomain.FilterOrder) ([]shareddomain.Order, error)
	Count(ctx context.Context, filter *shareddomain.FilterOrder) int
	// FetchAfter reads the orders matching filter with id > afterID in id order (keyset paging for exports)
	FetchAfter(ctx context.Context, filter *shareddomain.FilterOrder, afterID int64, limit int) ([]shareddomain.Order, error)
	Summary(ctx context.Context, filter *domain.FilterSummary) (domain.SummaryTotals, []domain.SummaryGroup, error)

	// ShiftTotals is the cash movement of a shift: cash sales landed in it, cash refunds paid out of it
	ShiftTotals(ctx context.Context, shiftID int64) (shareddomain.ShiftTotals, error)
}
