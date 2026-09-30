package usecase

import (
	"context"
	"slices"
	"time"

	"monorepo/globalshared/rest"
	"monorepo/services/payment/internal/modules/payment/domain"
	shareddomain "monorepo/services/payment/pkg/shared/domain"

	"github.com/golangid/candi/tracer"
)

func (uc *paymentUsecaseImpl) GetCheckout(ctx context.Context, token string) (res domain.ResponseCheckout, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PaymentUsecase:GetCheckout")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	p, err := uc.paymentByToken(ctx, token)
	if err != nil {
		return res, err
	}
	return uc.checkoutView(ctx, &p)
}

func (uc *paymentUsecaseImpl) checkoutView(ctx context.Context, p *shareddomain.Payment) (res domain.ResponseCheckout, err error) {
	res = domain.ResponseCheckout{
		PaymentID: p.ID, Status: p.Status, ReferenceID: p.ReferenceID, Description: p.Description,
		Amount: p.Amount, Fee: p.Fee, TotalAmount: p.TotalAmount, Currency: p.Currency,
		MethodCode: p.MethodCode, ExpiresAt: p.ExpiresAt.Format(time.RFC3339),
		SuccessURL: p.SuccessURL, FailureURL: p.FailureURL, Items: []shareddomain.Item{},
	}
	_ = p.Items.Decode(&res.Items)
	if p.PaidAt != nil {
		res.PaidAt = p.PaidAt.Format(time.RFC3339)
	}

	txns, err := uc.repoSQL.PaymentRepo().FetchTransactionsByPayment(ctx, p.ID)
	if err != nil {
		return res, err
	}
	// the customer sees the open attempt, or the one that got paid
	var shown *shareddomain.Transaction
	for i := range txns {
		if txns[i].Status == shareddomain.TransactionPending || txns[i].Status == shareddomain.TransactionPaid {
			shown = &txns[i]
		}
	}
	if shown != nil {
		res.Transaction = &domain.ResponseCheckoutTransaction{
			ID: shown.ID, Status: shown.Status, MethodCode: shown.MethodCode, Amount: shown.Amount,
			Instruction: decodeMap(shown.Instruction), ExpiresAt: shown.ExpiresAt.Format(time.RFC3339),
		}
	}
	return res, nil
}

func (uc *paymentUsecaseImpl) GetCheckoutMethods(ctx context.Context, token string) (res []domain.ResponseCheckoutMethod, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PaymentUsecase:GetCheckoutMethods")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	p, err := uc.paymentByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	methods, err := uc.usableMethods(ctx, &p)
	if err != nil {
		return nil, err
	}
	res = make([]domain.ResponseCheckoutMethod, 0, len(methods))
	for _, m := range methods {
		fee := m.CalculateFee(p.Amount)
		res = append(res, domain.ResponseCheckoutMethod{
			Code: m.Code, Name: m.Name, Type: m.Type, IconURL: m.IconURL,
			Fee: fee, TotalAmount: p.Amount + fee, Instructions: m.Instructions,
		})
	}
	return res, nil
}

// usableMethods are the methods the customer may pick right now: enabled, within the amount
// range, allowed by the caller, and backed by a gateway that is enabled and has credentials
func (uc *paymentUsecaseImpl) usableMethods(ctx context.Context, p *shareddomain.Payment) ([]shareddomain.Method, error) {
	methods, err := uc.repoSQL.MethodRepo().FetchEnabled(ctx)
	if err != nil {
		return nil, err
	}
	gateways, err := uc.repoSQL.GatewayRepo().FetchAll(ctx)
	if err != nil {
		return nil, err
	}
	usableGateway := map[string]bool{}
	for _, g := range gateways {
		usableGateway[g.Code] = g.IsEnabled && (g.CredentialsEnc != "" || g.Code == shareddomain.GatewayMock)
	}
	var allowed []string
	_ = p.AllowedMethods.Decode(&allowed)

	out := make([]shareddomain.Method, 0, len(methods))
	for _, m := range methods {
		if len(allowed) > 0 && !slices.Contains(allowed, m.Code) {
			continue
		}
		if !m.AmountAllowed(p.Amount) {
			continue
		}
		if !m.IsCash() && (m.GatewayCode == nil || !usableGateway[*m.GatewayCode]) {
			continue
		}
		out = append(out, m)
	}
	return out, nil
}

func (uc *paymentUsecaseImpl) usableMethod(ctx context.Context, p *shareddomain.Payment, code string) (shareddomain.Method, error) {
	methods, err := uc.usableMethods(ctx, p)
	if err != nil {
		return shareddomain.Method{}, err
	}
	for _, m := range methods {
		if m.Code == code {
			return m, nil
		}
	}
	return shareddomain.Method{}, rest.NewInvalid("payment method " + code + " is not available for this payment")
}

func (uc *paymentUsecaseImpl) notPayable(p *shareddomain.Payment) error {
	if p.IsFinal() {
		return rest.NewConflict("payment is " + p.Status)
	}
	return nil
}

func (uc *paymentUsecaseImpl) SelectMethod(ctx context.Context, token, methodCode string) (res domain.ResponseCheckout, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PaymentUsecase:SelectMethod")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	p, err := uc.paymentByToken(ctx, token)
	if err != nil {
		return res, err
	}
	if err = uc.notPayable(&p); err != nil {
		return res, err
	}
	method, err := uc.usableMethod(ctx, &p, methodCode)
	if err != nil {
		return res, err
	}
	if p.Status == shareddomain.PaymentProcessing && p.MethodCode == methodCode {
		return uc.checkoutView(ctx, &p) // same method again: keep the running attempt
	}

	var dropped []shareddomain.Transaction
	err = uc.repoSQL.WithTransaction(ctx, func(ctx context.Context) error {
		cur, err := uc.repoSQL.PaymentRepo().LockPayment(ctx, p.ID)
		if err != nil {
			return err
		}
		if err = uc.notPayable(&cur); err != nil {
			return err
		}
		if open, ok := uc.openTransaction(ctx, cur.ID); ok {
			if err = uc.closeTransaction(ctx, &open, shareddomain.TransactionCancelled, "customer changed payment method"); err != nil {
				return err
			}
			dropped = append(dropped, open)
		}
		fee := method.CalculateFee(cur.Amount)
		cur.Status, cur.MethodCode, cur.Fee, cur.TotalAmount = shareddomain.PaymentPending, method.Code, fee, cur.Amount+fee
		p = cur
		return uc.repoSQL.PaymentRepo().SavePayment(ctx, &cur)
	})
	if err != nil {
		return res, err
	}
	uc.cancelAtGateway(ctx, dropped...)
	return uc.checkoutView(ctx, &p)
}

func (uc *paymentUsecaseImpl) CancelCheckout(ctx context.Context, token string) (res domain.ResponseCheckout, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PaymentUsecase:CancelCheckout")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	p, err := uc.paymentByToken(ctx, token)
	if err != nil {
		return res, err
	}
	if p, err = uc.cancelPayment(ctx, p.ID, "cancelled by customer"); err != nil {
		return res, err
	}
	return uc.checkoutView(ctx, &p)
}

func (uc *paymentUsecaseImpl) CancelPayment(ctx context.Context, id string) (res domain.ResponsePayment, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PaymentUsecase:CancelPayment")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	if !isValidUUID(id) {
		return res, rest.NewNotFound("payment not found")
	}
	if _, err = uc.cancelPayment(ctx, id, "cancelled by caller"); err != nil {
		return res, err
	}
	return uc.GetPayment(ctx, id)
}

// cancelPayment moves an open payment to cancelled; cancelling twice is a no-op
func (uc *paymentUsecaseImpl) cancelPayment(ctx context.Context, id, reason string) (p shareddomain.Payment, err error) {
	var dropped []shareddomain.Transaction
	err = uc.repoSQL.WithTransaction(ctx, func(ctx context.Context) error {
		cur, err := uc.repoSQL.PaymentRepo().LockPayment(ctx, id)
		if err != nil {
			return rest.NewNotFound("payment not found")
		}
		p = cur
		switch cur.Status {
		case shareddomain.PaymentCancelled:
			return nil
		case shareddomain.PaymentPaid, shareddomain.PaymentExpired:
			return rest.NewConflict("payment is already " + cur.Status)
		}
		open, hasOpen := uc.openTransaction(ctx, cur.ID)
		if hasOpen {
			if err = uc.closeTransaction(ctx, &open, shareddomain.TransactionCancelled, reason); err != nil {
				return err
			}
			dropped = append(dropped, open)
		}
		cur.Status = shareddomain.PaymentCancelled
		if err = uc.repoSQL.PaymentRepo().SavePayment(ctx, &cur); err != nil {
			return err
		}
		p = cur
		return uc.enqueueEvent(ctx, shareddomain.EventPaymentCancelled, &cur, nil)
	})
	if err != nil {
		return p, err
	}
	uc.cancelAtGateway(ctx, dropped...)
	uc.afterCommit()
	return p, nil
}
