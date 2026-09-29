package usecase

import (
	"context"

	shareddomain "monorepo/services/payment/pkg/shared/domain"

	"github.com/golangid/candi/logger"
	"github.com/golangid/candi/tracer"
)

const expireBatch = 100

// ExpireOverduePayments moves open payments past their expiry to expired (cron job)
func (uc *paymentUsecaseImpl) ExpireOverduePayments(ctx context.Context) (expired int, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PaymentUsecase:ExpireOverduePayments")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	ids, err := uc.repoSQL.PaymentRepo().FetchExpiredPaymentIDs(ctx, uc.now(), expireBatch)
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		if perr := uc.expirePayment(ctx, id); perr != nil {
			logger.LogIf("payment: expire %s failed: %v", id, perr)
			err = perr // keep going, report the last failure
			continue
		}
		expired++
	}
	return expired, err
}

// expirePayment expires one payment if it is still open and overdue. Idempotent.
func (uc *paymentUsecaseImpl) expirePayment(ctx context.Context, id string) error {
	var dropped []shareddomain.Transaction
	err := uc.repoSQL.WithTransaction(ctx, func(ctx context.Context) error {
		p, err := uc.repoSQL.PaymentRepo().LockPayment(ctx, id)
		if err != nil {
			return err
		}
		// re-check under the lock: it may have been paid or cancelled since the caller looked
		if p.IsFinal() || !uc.now().After(p.ExpiresAt) {
			return nil
		}
		if open, ok := uc.openTransaction(ctx, p.ID); ok {
			if err = uc.closeTransaction(ctx, &open, shareddomain.TransactionExpired, "payment expired"); err != nil {
				return err
			}
			dropped = append(dropped, open)
		}
		p.Status = shareddomain.PaymentExpired
		if err = uc.repoSQL.PaymentRepo().SavePayment(ctx, &p); err != nil {
			return err
		}
		return uc.enqueueEvent(ctx, shareddomain.EventPaymentExpired, &p, nil)
	})
	if err != nil {
		return err
	}
	uc.cancelAtGateway(ctx, dropped...)
	uc.afterCommit()
	return nil
}
