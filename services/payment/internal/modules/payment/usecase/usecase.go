package usecase

import (
	"context"
	"sync"
	"time"

	"monorepo/services/payment/internal/modules/gateway/provider"
	"monorepo/services/payment/internal/modules/payment/domain"
	"monorepo/services/payment/pkg/shared"
	shareddomain "monorepo/services/payment/pkg/shared/domain"
	"monorepo/services/payment/pkg/shared/repository"
	"monorepo/services/payment/pkg/shared/usecase/common"

	"github.com/golangid/candi/codebase/factory/dependency"
	"github.com/golangid/candi/codebase/factory/types"
	"github.com/golangid/candi/codebase/interfaces"
)

// PaymentUsecase abstraction
type PaymentUsecase interface {
	// Caller side (other services)
	CreatePayment(ctx context.Context, source string, req *domain.RequestCreatePayment) (res domain.ResponseCreatePayment, err error)
	GetPayment(ctx context.Context, id string) (res domain.ResponsePayment, err error)
	GetAllPayments(ctx context.Context, filter *domain.FilterPayment) (res domain.ResponsePaymentList, err error)
	CancelPayment(ctx context.Context, id string) (res domain.ResponsePayment, err error)

	// Checkout journey (the customer, authenticated by the payment token)
	GetCheckout(ctx context.Context, token string) (res domain.ResponseCheckout, err error)
	GetCheckoutMethods(ctx context.Context, token string) (res []domain.ResponseCheckoutMethod, err error)
	SelectMethod(ctx context.Context, token, methodCode string) (res domain.ResponseCheckout, err error)
	Pay(ctx context.Context, token, methodCode string) (res domain.ResponseCheckout, err error)
	CancelCheckout(ctx context.Context, token string) (res domain.ResponseCheckout, err error)

	// Cash (a cashier)
	GetCashPayment(ctx context.Context, cashCode string) (res domain.ResponseCash, err error)
	ConfirmCashPayment(ctx context.Context, cashCode, actor string, req *domain.RequestConfirmCash) (res domain.ResponseCashConfirm, err error)

	// Gateway callbacks consumed from Kafka
	HandleCallback(ctx context.Context, msg *domain.CallbackMessage) (out domain.CallbackOutcome, err error)
	// RecordCallbackFailure logs a callback that could not be processed after retries
	RecordCallbackFailure(ctx context.Context, msg *domain.CallbackMessage, cause error)
	ReplayCallback(ctx context.Context, logID int) (out domain.CallbackOutcome, err error)
	GetAllCallbackLogs(ctx context.Context, filter *domain.FilterCallbackLog) (res domain.ResponseCallbackLogList, err error)
	GetCallbackLog(ctx context.Context, id int) (res shareddomain.CallbackLog, err error)
	// ConsumeTopics lists the topic names the callback consumer must subscribe to right now
	ConsumeTopics(ctx context.Context) (topics []string, err error)
	// SimulateMockCallback publishes a callback of the dev-only mock gateway to its Kafka topic
	SimulateMockCallback(ctx context.Context, req *domain.RequestMockCallback) (err error)

	// Background jobs (cron)
	ExpireOverduePayments(ctx context.Context) (expired int, err error)
	FlushOutbox(ctx context.Context) (published int, err error)
}

type paymentUsecaseImpl struct {
	deps          dependency.Dependency
	sharedUsecase common.Usecase
	repoSQL       repository.RepoSQL

	// swappable in tests
	now         func() time.Time
	newID       func() string
	newToken    func() string
	newCashCode func() string
	env         func() shared.Environment
	publisher   func() interfaces.Publisher
	// providers resolves a gateway row to its provider; requireEnabled=false is used for
	// callbacks, which must still settle after a gateway was disabled
	providers func(row shareddomain.Gateway, requireEnabled bool) (provider.Provider, provider.Config, error)
	// afterCommit runs after a state change that queued outbox events
	afterCommit func()

	flushMu sync.Mutex
}

// NewPaymentUsecase usecase impl constructor
func NewPaymentUsecase(deps dependency.Dependency) (PaymentUsecase, func(sharedUsecase common.Usecase)) {
	uc := &paymentUsecaseImpl{
		deps:        deps,
		repoSQL:     repository.GetSharedRepoSQL(),
		now:         time.Now,
		newID:       newUUID,
		newToken:    newToken,
		newCashCode: newCashCode,
		env:         shared.GetEnv,
		providers:   defaultProviders,
	}
	uc.publisher = func() interfaces.Publisher {
		if b := deps.GetBroker(types.Kafka); b != nil {
			return b.GetPublisher()
		}
		return nil
	}
	uc.afterCommit = uc.kickOutbox
	return uc, func(sharedUsecase common.Usecase) {
		uc.sharedUsecase = sharedUsecase
	}
}
