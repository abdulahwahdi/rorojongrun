package usecase

import (
	"context"
	"errors"

	"monorepo/globalshared/rest"
	"monorepo/services/payment/internal/modules/payment/domain"
	shareddomain "monorepo/services/payment/pkg/shared/domain"

	"github.com/golangid/candi/candishared"
	"github.com/golangid/candi/tracer"
	"gorm.io/gorm"
)

func (uc *paymentUsecaseImpl) GetPayment(ctx context.Context, id string) (res domain.ResponsePayment, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PaymentUsecase:GetPayment")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	if !isValidUUID(id) {
		return res, rest.NewNotFound("payment not found")
	}
	p, err := uc.loadPayment(ctx, id)
	if err != nil {
		return res, err
	}
	res.Payment = p
	if res.Transactions, err = uc.repoSQL.PaymentRepo().FetchTransactionsByPayment(ctx, id); err != nil {
		return res, err
	}
	if res.Transactions == nil {
		res.Transactions = []shareddomain.Transaction{}
	}
	return res, nil
}

func (uc *paymentUsecaseImpl) GetAllPayments(ctx context.Context, filter *domain.FilterPayment) (res domain.ResponsePaymentList, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PaymentUsecase:GetAllPayments")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	repo := uc.repoSQL.PaymentRepo()
	if res.Data, err = repo.FetchAllPayments(ctx, filter); err != nil {
		return res, err
	}
	if res.Data == nil {
		res.Data = []shareddomain.Payment{}
	}
	res.Meta = candishared.NewMeta(filter.Page, filter.Limit, repo.CountPayments(ctx, filter))
	return res, nil
}

// loadPayment reads a payment by id and applies expiry lazily, so a caller never sees
// an overdue payment as still payable while the cron has not run yet
func (uc *paymentUsecaseImpl) loadPayment(ctx context.Context, id string) (shareddomain.Payment, error) {
	p, err := uc.repoSQL.PaymentRepo().FindPaymentByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return p, rest.NewNotFound("payment not found")
	}
	if err != nil {
		return p, err
	}
	return uc.settleExpiry(ctx, p)
}

func (uc *paymentUsecaseImpl) paymentByToken(ctx context.Context, token string) (shareddomain.Payment, error) {
	p, err := uc.repoSQL.PaymentRepo().FindPaymentByToken(ctx, token)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return p, rest.NewNotFound("payment not found")
	}
	if err != nil {
		return p, err
	}
	return uc.settleExpiry(ctx, p)
}

func (uc *paymentUsecaseImpl) settleExpiry(ctx context.Context, p shareddomain.Payment) (shareddomain.Payment, error) {
	if p.IsFinal() || !uc.now().After(p.ExpiresAt) {
		return p, nil
	}
	if err := uc.expirePayment(ctx, p.ID); err != nil {
		return p, err
	}
	return uc.repoSQL.PaymentRepo().FindPaymentByID(ctx, p.ID)
}
