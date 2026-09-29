package usecase

import (
	"context"
	"encoding/json"
	"strings"

	"monorepo/globalshared/rest"
	"monorepo/services/payment/internal/modules/gateway/provider"
	"monorepo/services/payment/internal/modules/payment/domain"
	shareddomain "monorepo/services/payment/pkg/shared/domain"

	"github.com/golangid/candi/logger"
	"github.com/golangid/candi/tracer"
)

// Pay starts a checkout attempt for the selected method (methodCode may be given here instead
// of a prior SelectMethod call). It is idempotent: repeating it for the running attempt of the
// same method returns that attempt instead of creating another charge.
//
// Every attempt, cash or gateway, queues a payment.checkout_started event in the same
// transaction that creates it, so no checkout is ever missed on Kafka. The gateway call
// happens after that transaction commits (never while holding the row lock).
func (uc *paymentUsecaseImpl) Pay(ctx context.Context, token, methodCode string) (res domain.ResponseCheckout, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PaymentUsecase:Pay")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	p, err := uc.paymentByToken(ctx, token)
	if err != nil {
		return res, err
	}
	if err = uc.notPayable(&p); err != nil {
		return res, err
	}
	code := methodCode
	if code == "" {
		code = p.MethodCode
	}
	if code == "" {
		return res, rest.NewInvalid("select a payment method first")
	}
	method, err := uc.usableMethod(ctx, &p, code)
	if err != nil {
		return res, err
	}
	if p.Status == shareddomain.PaymentProcessing && p.MethodCode == code {
		if _, ok := uc.openTransaction(ctx, p.ID); ok {
			return uc.checkoutView(ctx, &p)
		}
	}

	var (
		prov provider.Provider
		cfg  provider.Config
		gw   string
	)
	if !method.IsCash() {
		gw = *method.GatewayCode
		row, gerr := uc.repoSQL.GatewayRepo().FindByCode(ctx, gw)
		if gerr != nil {
			return res, gerr
		}
		if prov, cfg, err = uc.providers(row, true); err != nil {
			logger.LogE("payment: gateway " + gw + " unusable: " + err.Error())
			return res, rest.NewUnavailable("this payment method is temporarily unavailable")
		}
	}

	// step 1: create the attempt (and its events) atomically
	var (
		txn     shareddomain.Transaction
		reused  bool
		dropped []shareddomain.Transaction
	)
	err = uc.repoSQL.WithTransaction(ctx, func(ctx context.Context) error {
		repo := uc.repoSQL.PaymentRepo()
		cur, err := repo.LockPayment(ctx, p.ID)
		if err != nil {
			return err
		}
		if err = uc.notPayable(&cur); err != nil {
			return err
		}
		if !uc.now().Before(cur.ExpiresAt) {
			return rest.NewConflict("payment link expired")
		}
		open, hasOpen := uc.openTransaction(ctx, cur.ID)
		if hasOpen && open.MethodCode == code {
			txn, reused, p = open, true, cur // a concurrent Pay already created it
			return nil
		}
		if hasOpen {
			if err = uc.closeTransaction(ctx, &open, shareddomain.TransactionCancelled, "customer changed payment method"); err != nil {
				return err
			}
			dropped = append(dropped, open)
		}

		fee := method.CalculateFee(cur.Amount)
		txn = shareddomain.Transaction{
			ID: uc.newID(), PaymentID: cur.ID, MethodCode: code, GatewayCode: gw,
			Status: shareddomain.TransactionPending, Amount: cur.Amount + fee,
			Instruction: shareddomain.NewJSON(nil), GatewayResponse: shareddomain.NewJSON(nil),
			ExpiresAt: cur.ExpiresAt,
		}
		if method.IsCash() {
			cashCode := uc.newCashCode()
			txn.CashCode = &cashCode
			txn.Instruction = shareddomain.NewJSON(map[string]any{
				"type": shareddomain.MethodCash, "cashCode": cashCode, "instructions": method.Instructions,
			})
		}
		if err = repo.SaveTransaction(ctx, &txn); err != nil {
			return err
		}
		cur.Status, cur.MethodCode, cur.Fee, cur.TotalAmount = shareddomain.PaymentProcessing, code, fee, cur.Amount+fee
		if err = repo.SavePayment(ctx, &cur); err != nil {
			return err
		}
		if err = uc.enqueueEvent(ctx, shareddomain.EventCheckoutStarted, &cur, &txn); err != nil {
			return err
		}
		if method.IsCash() { // nothing to wait for: the instruction is already known
			if err = uc.enqueueEmail(ctx, domain.TemplateCheckout, &cur, &txn); err != nil {
				return err
			}
		}
		p = cur
		return nil
	})
	if err != nil {
		return res, rest.MapDBError(err)
	}
	uc.cancelAtGateway(ctx, dropped...)
	uc.afterCommit()
	if reused || method.IsCash() {
		return uc.checkoutView(ctx, &p)
	}

	// step 2: create the charge at the gateway
	charge, cerr := prov.CreateCharge(ctx, cfg, uc.chargeRequest(&p, &txn, method))
	if cerr != nil {
		logger.LogE("payment: charge " + txn.ID + " at " + gw + " failed: " + cerr.Error())
		if ferr := uc.failAttempt(ctx, p.ID, txn.ID, truncateReason(cerr.Error()), charge.Raw); ferr != nil {
			logger.LogE("payment: cannot record failed attempt " + txn.ID + ": " + ferr.Error())
		}
		return res, rest.NewBadGateway("the payment gateway could not create this payment, please retry or choose another method")
	}

	// step 3: keep the instruction and tell the customer
	var lateCancel *shareddomain.Transaction
	err = uc.repoSQL.WithTransaction(ctx, func(ctx context.Context) error {
		repo := uc.repoSQL.PaymentRepo()
		cur, err := repo.LockPayment(ctx, p.ID)
		if err != nil {
			return err
		}
		cur2, err := repo.FindTransactionByID(ctx, txn.ID)
		if err != nil {
			return err
		}
		txn = cur2
		txn.Instruction = shareddomain.NewJSON(charge.Instruction)
		txn.GatewayResponse = shareddomain.NewJSON(map[string]any{"externalRef": charge.ExternalRef, "raw": rawOrNil(charge.Raw)})
		if txn.Status != shareddomain.TransactionPending {
			// the customer switched method or the payment ended while the gateway was busy
			lateCancel = &txn
			return repo.SaveTransaction(ctx, &txn)
		}
		if err = repo.SaveTransaction(ctx, &txn); err != nil {
			return err
		}
		p = cur
		return uc.enqueueEmail(ctx, domain.TemplateCheckout, &cur, &txn)
	})
	if err != nil {
		return res, err
	}
	if lateCancel != nil {
		uc.cancelAtGateway(ctx, *lateCancel)
		return res, rest.NewConflict("this checkout attempt was replaced, please retry")
	}
	uc.afterCommit()
	return uc.checkoutView(ctx, &p)
}

func (uc *paymentUsecaseImpl) chargeRequest(p *shareddomain.Payment, txn *shareddomain.Transaction, m shareddomain.Method) provider.ChargeRequest {
	var customer shareddomain.Customer
	_ = p.Customer.Decode(&customer)
	items := []shareddomain.Item{}
	_ = p.Items.Decode(&items)
	if fee := txn.Amount - p.Amount; fee > 0 {
		items = append(items, shareddomain.Item{Name: "Payment fee", Price: fee, Quantity: 1})
	}
	return provider.ChargeRequest{
		TransactionID: txn.ID, Amount: txn.Amount, Currency: p.Currency,
		Method:   provider.MethodInfo{Code: m.Code, Type: m.Type, Channel: m.GatewayChannel},
		Customer: customer, Items: items, Description: p.Description, ExpiresAt: txn.ExpiresAt,
		SuccessURL: p.SuccessURL, FailureURL: p.FailureURL,
	}
}

// failAttempt records a gateway rejection: the attempt fails and the payment can be paid another way
func (uc *paymentUsecaseImpl) failAttempt(ctx context.Context, paymentID, txnID, reason string, raw []byte) error {
	return uc.repoSQL.WithTransaction(ctx, func(ctx context.Context) error {
		repo := uc.repoSQL.PaymentRepo()
		p, err := repo.LockPayment(ctx, paymentID)
		if err != nil {
			return err
		}
		txn, err := repo.FindTransactionByID(ctx, txnID)
		if err != nil {
			return err
		}
		if txn.Status != shareddomain.TransactionPending {
			return nil
		}
		txn.GatewayResponse = shareddomain.NewJSON(map[string]any{"raw": rawOrNil(raw)})
		if err = uc.closeTransaction(ctx, &txn, shareddomain.TransactionFailed, reason); err != nil {
			return err
		}
		if p.Status == shareddomain.PaymentProcessing {
			if _, other := uc.openTransaction(ctx, p.ID); !other {
				p.Status = shareddomain.PaymentPending
				return repo.SavePayment(ctx, &p)
			}
		}
		return nil
	})
}

func rawOrNil(raw []byte) any {
	if len(raw) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	return v
}

func truncateReason(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 480 {
		return s[:480]
	}
	return s
}
