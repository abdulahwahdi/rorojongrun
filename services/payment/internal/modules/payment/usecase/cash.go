package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"monorepo/globalshared/rest"
	"monorepo/services/payment/internal/modules/payment/domain"
	"monorepo/services/payment/pkg/helper"
	shareddomain "monorepo/services/payment/pkg/shared/domain"

	"github.com/golangid/candi/tracer"
	"gorm.io/gorm"
)

func (uc *paymentUsecaseImpl) cashResponse(p *shareddomain.Payment, txn *shareddomain.Transaction) domain.ResponseCash {
	return domain.ResponseCash{
		CashCode: helper.StrVal(txn.CashCode), PaymentID: p.ID, ReferenceID: p.ReferenceID, Source: p.Source,
		Status: p.Status, TotalAmount: txn.Amount, Currency: p.Currency, ExpiresAt: p.ExpiresAt.Format(time.RFC3339),
	}
}

// GetCashPayment lets a cashier look up what a customer must pay for a cash code
func (uc *paymentUsecaseImpl) GetCashPayment(ctx context.Context, cashCode string) (res domain.ResponseCash, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PaymentUsecase:GetCashPayment")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	txn, err := uc.repoSQL.PaymentRepo().FindTransactionByCashCode(ctx, strings.ToUpper(strings.TrimSpace(cashCode)))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return res, rest.NewNotFound("cash code not found")
	}
	if err != nil {
		return res, err
	}
	p, err := uc.loadPayment(ctx, txn.PaymentID)
	if err != nil {
		return res, err
	}
	return uc.cashResponse(&p, &txn), nil
}

// ConfirmCashPayment records that the cashier received the money. The amount received must
// cover the total due; the difference is reported as change.
func (uc *paymentUsecaseImpl) ConfirmCashPayment(ctx context.Context, cashCode, actor string, req *domain.RequestConfirmCash) (res domain.ResponseCashConfirm, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PaymentUsecase:ConfirmCashPayment")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	cashCode = strings.ToUpper(strings.TrimSpace(cashCode))
	repo := uc.repoSQL.PaymentRepo()
	found, err := repo.FindTransactionByCashCode(ctx, cashCode)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return res, rest.NewNotFound("cash code not found")
	}
	if err != nil {
		return res, err
	}
	if _, err = uc.loadPayment(ctx, found.PaymentID); err != nil { // applies expiry
		return res, err
	}

	var (
		p       shareddomain.Payment
		txn     shareddomain.Transaction
		dropped []shareddomain.Transaction
	)
	err = uc.repoSQL.WithTransaction(ctx, func(ctx context.Context) error {
		cur, err := repo.LockPayment(ctx, found.PaymentID)
		if err != nil {
			return err
		}
		t, err := repo.FindTransactionByID(ctx, found.ID)
		if err != nil {
			return err
		}
		switch {
		case t.Status == shareddomain.TransactionPaid:
			return rest.NewConflict("this cash code was already confirmed")
		case t.Status != shareddomain.TransactionPending || cur.IsFinal():
			return rest.NewConflict("this cash code is no longer payable (payment " + cur.Status + ")")
		case req.AmountReceived < t.Amount:
			return rest.NewInvalid("the amount received is less than the amount due")
		}
		now := uc.now()
		t.CashReceived, t.ConfirmedBy = &req.AmountReceived, &actor
		if dropped, err = uc.markPaid(ctx, &cur, &t, now); err != nil {
			return err
		}
		p, txn = cur, t
		return nil
	})
	if err != nil {
		return res, err
	}
	uc.cancelAtGateway(ctx, dropped...)
	uc.afterCommit()
	return domain.ResponseCashConfirm{
		ResponseCash: uc.cashResponse(&p, &txn), AmountReceived: req.AmountReceived, Change: req.AmountReceived - txn.Amount,
	}, nil
}
