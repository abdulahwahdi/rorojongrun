package repository

import (
	"context"
	"time"

	"monorepo/services/payment/internal/modules/payment/domain"
	shareddomain "monorepo/services/payment/pkg/shared/domain"
)

// PaymentRepository abstract interface. It covers the payment aggregate: payments,
// their checkout attempts (transactions), callback logs and the outbox.
type PaymentRepository interface {
	// Payments
	SavePayment(ctx context.Context, data *shareddomain.Payment) error
	FindPaymentByID(ctx context.Context, id string) (shareddomain.Payment, error)
	FindPaymentByToken(ctx context.Context, token string) (shareddomain.Payment, error)
	// FindActivePaymentByRef finds the pending/processing payment of a caller's reference
	FindActivePaymentByRef(ctx context.Context, source, referenceID string) (shareddomain.Payment, error)
	// LockPayment reads a payment with SELECT ... FOR UPDATE; call inside WithTransaction
	LockPayment(ctx context.Context, id string) (shareddomain.Payment, error)
	FetchAllPayments(ctx context.Context, filter *domain.FilterPayment) ([]shareddomain.Payment, error)
	CountPayments(ctx context.Context, filter *domain.FilterPayment) int
	// FetchExpiredPaymentIDs returns open payments whose expiry passed
	FetchExpiredPaymentIDs(ctx context.Context, now time.Time, limit int) ([]string, error)

	// Transactions
	SaveTransaction(ctx context.Context, data *shareddomain.Transaction) error
	FindTransactionByID(ctx context.Context, id string) (shareddomain.Transaction, error)
	FindTransactionByCashCode(ctx context.Context, cashCode string) (shareddomain.Transaction, error)
	FetchTransactionsByPayment(ctx context.Context, paymentID string) ([]shareddomain.Transaction, error)
	// FindOpenTransaction returns the pending attempt of a payment
	FindOpenTransaction(ctx context.Context, paymentID string) (shareddomain.Transaction, error)

	// Callback logs
	SaveCallbackLog(ctx context.Context, data *shareddomain.CallbackLog) error
	FindCallbackLogByID(ctx context.Context, id int) (shareddomain.CallbackLog, error)
	FetchAllCallbackLogs(ctx context.Context, filter *domain.FilterCallbackLog) ([]shareddomain.CallbackLog, error)
	CountCallbackLogs(ctx context.Context, filter *domain.FilterCallbackLog) int

	// Outbox
	SaveOutbox(ctx context.Context, data *shareddomain.Outbox) error
	// LockPendingOutbox reads unpublished events with FOR UPDATE SKIP LOCKED; call inside WithTransaction
	LockPendingOutbox(ctx context.Context, limit int) ([]shareddomain.Outbox, error)
}
