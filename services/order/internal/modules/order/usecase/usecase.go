package usecase

import (
	"context"
	"sync"
	"time"

	"monorepo/services/order/internal/modules/order/domain"
	"monorepo/services/order/pkg/shared"
	shareddomain "monorepo/services/order/pkg/shared/domain"
	"monorepo/services/order/pkg/shared/repository"
	"monorepo/services/order/pkg/shared/usecase/common"

	"github.com/golangid/candi/codebase/factory/dependency"
	"github.com/golangid/candi/codebase/factory/types"
	"github.com/golangid/candi/codebase/interfaces"
)

// OrderUsecase abstraction: the sales ledger built from payment events, the order lifecycle and reports
type OrderUsecase interface {
	// RecordPaymentEvent books a payment.* event: creates the order on the first event of a payment,
	// updates its payment status, and moves the order out of awaiting_payment when the payment ends
	RecordPaymentEvent(ctx context.Context, ev *domain.PaymentEvent, src domain.KafkaSource) (err error)

	GetAllOrders(ctx context.Context, filter *shareddomain.FilterOrder) (res domain.ResponseOrderList, err error)
	// GetOrder finds an order by numeric id or order number
	GetOrder(ctx context.Context, idOrNumber string) (res domain.ResponseOrder, err error)
	GetSummary(ctx context.Context, filter *domain.FilterSummary) (res domain.ResponseSummary, err error)
	// ResolveDates turns the DateFrom / DateTo strings of a filter into times in the merchant's timezone
	ResolveDates(ctx context.Context, merchantID string, r *shareddomain.DateRange) (tz string, err error)

	// UpdateStatus moves the order status (staff)
	UpdateStatus(ctx context.Context, idOrNumber, actor string, req *domain.RequestUpdateStatus) (res domain.ResponseOrder, err error)
	// OverridePaymentStatus sets the payment status by hand (admin); the payment service is not changed
	OverridePaymentStatus(ctx context.Context, idOrNumber, actor string, req *domain.RequestOverridePaymentStatus) (res domain.ResponseOrder, err error)

	// Outbox (shared with the other modules through common.Usecase)
	Enqueue(ctx context.Context, eventType, key string, payload any) error
	LogActivity(ctx context.Context, entry shareddomain.Activity) error
	KickOutbox()
	FlushOutbox(ctx context.Context) (published int, err error)
}

type orderUsecaseImpl struct {
	deps          dependency.Dependency
	sharedUsecase common.Usecase
	repoSQL       repository.RepoSQL

	// swappable in tests
	now         func() time.Time
	env         func() shared.Environment
	publisher   func() interfaces.Publisher
	afterCommit func()

	flushMu sync.Mutex
}

// NewOrderUsecase usecase impl constructor
func NewOrderUsecase(deps dependency.Dependency) (OrderUsecase, func(sharedUsecase common.Usecase)) {
	uc := &orderUsecaseImpl{
		deps:    deps,
		repoSQL: repository.GetSharedRepoSQL(),
		now:     time.Now,
		env:     shared.GetEnv,
	}
	uc.publisher = func() interfaces.Publisher {
		if deps == nil {
			return nil
		}
		if b := deps.GetBroker(types.Kafka); b != nil {
			return b.GetPublisher()
		}
		return nil
	}
	uc.afterCommit = uc.KickOutbox
	return uc, func(sharedUsecase common.Usecase) {
		uc.sharedUsecase = sharedUsecase
	}
}
