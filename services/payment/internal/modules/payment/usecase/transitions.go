package usecase

import (
	"context"
	"monorepo/services/payment/internal/modules/payment/domain"
	"time"

	shareddomain "monorepo/services/payment/pkg/shared/domain"

	"github.com/golangid/candi/logger"
)

// openTransaction returns the pending attempt of a payment, if any
func (uc *paymentUsecaseImpl) openTransaction(ctx context.Context, paymentID string) (txn shareddomain.Transaction, ok bool) {
	txn, err := uc.repoSQL.PaymentRepo().FindOpenTransaction(ctx, paymentID)
	return txn, err == nil
}

// closeTransaction ends a pending attempt with a terminal status
func (uc *paymentUsecaseImpl) closeTransaction(ctx context.Context, txn *shareddomain.Transaction, status, reason string) error {
	txn.Status, txn.FailureReason = status, reason
	return uc.repoSQL.PaymentRepo().SaveTransaction(ctx, txn)
}

// markPaid settles a payment with a transaction the customer paid: it must run inside the
// transaction that holds the payment lock. The payment takes the paid attempt's method and
// fee. Other open attempts are cancelled and returned so the caller can cancel them at
// the gateway after commit.
func (uc *paymentUsecaseImpl) markPaid(ctx context.Context, p *shareddomain.Payment, txn *shareddomain.Transaction, paidAt time.Time) (cancelled []shareddomain.Transaction, err error) {
	repo := uc.repoSQL.PaymentRepo()

	txn.Status, txn.PaidAt = shareddomain.TransactionPaid, &paidAt
	if err = repo.SaveTransaction(ctx, txn); err != nil {
		return nil, err
	}
	if other, ok := uc.openTransaction(ctx, p.ID); ok && other.ID != txn.ID {
		if err = uc.closeTransaction(ctx, &other, shareddomain.TransactionCancelled, "payment settled by another attempt"); err != nil {
			return nil, err
		}
		cancelled = append(cancelled, other)
	}

	p.Status, p.PaidAt, p.MethodCode = shareddomain.PaymentPaid, &paidAt, txn.MethodCode
	p.Fee, p.TotalAmount = txn.Amount-p.Amount, txn.Amount
	if err = repo.SavePayment(ctx, p); err != nil {
		return nil, err
	}
	if err = uc.enqueueEvent(ctx, shareddomain.EventPaymentCompleted, p, txn); err != nil {
		return nil, err
	}
	return cancelled, uc.enqueueEmail(ctx, domain.TemplatePaid, p, txn)
}

// cancelAtGateway asks the gateway to drop a charge. Best effort: the local state is already
// final, a gateway that keeps the charge alive is caught by the amount/paid checks of callbacks.
func (uc *paymentUsecaseImpl) cancelAtGateway(ctx context.Context, txns ...shareddomain.Transaction) {
	for _, txn := range txns {
		if txn.GatewayCode == "" {
			continue
		}
		row, err := uc.repoSQL.GatewayRepo().FindByCode(ctx, txn.GatewayCode)
		if err != nil {
			continue
		}
		prov, cfg, err := uc.providers(row, false)
		if err != nil {
			logger.LogIf("payment: cannot cancel %s at %s: %v", txn.ID, txn.GatewayCode, err)
			continue
		}
		ref := decodeMap(txn.GatewayResponse)["externalRef"]
		refStr, _ := ref.(string)
		if err := prov.Cancel(ctx, cfg, txn.ID, refStr); err != nil {
			logger.LogIf("payment: gateway cancel of %s failed (ignored): %v", txn.ID, err)
		}
	}
}
