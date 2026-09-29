package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"monorepo/globalshared/rest"
	"monorepo/services/payment/internal/modules/gateway/provider"
	"monorepo/services/payment/internal/modules/payment/domain"
	shareddomain "monorepo/services/payment/pkg/shared/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var bg = context.Background()

func eventOf(t *testing.T, o shareddomain.Outbox) domain.PaymentEvent {
	t.Helper()
	var ev domain.PaymentEvent
	require.NoError(t, json.Unmarshal(o.Payload, &ev))
	return ev
}

func appErrStatus(err error) int {
	var app *rest.AppError
	if errors.As(err, &app) {
		return app.Status
	}
	return 0
}

func Test_CreatePayment(t *testing.T) {
	t.Run("creates a link and queues payment.created", func(t *testing.T) {
		h := newHarness(t)
		res := h.create(t, "ORD-1", 50000)

		assert.Equal(t, "pending", res.Status)
		assert.Equal(t, "https://pay.example.com/pay/"+res.Token, res.PaymentURL)
		assert.Equal(t, h.now.Add(24*time.Hour).Format(time.RFC3339), res.ExpiresAt)

		evs := h.repo.pay.events("payment.created")
		require.Len(t, evs, 1)
		ev := eventOf(t, evs[0])
		assert.Equal(t, "order", ev.Source)
		assert.Equal(t, "ORD-1", ev.ReferenceID)
		assert.EqualValues(t, 50000, ev.Amount)
		assert.Equal(t, 1, h.kicks, "outbox flush is triggered after commit")
	})

	t.Run("idempotent per source and reference while open", func(t *testing.T) {
		h := newHarness(t)
		first := h.create(t, "ORD-1", 50000)
		second := h.create(t, "ORD-1", 50000)
		assert.Equal(t, first.PaymentID, second.PaymentID)
		assert.Equal(t, first.Token, second.Token)
		assert.Len(t, h.repo.pay.events("payment.created"), 1)

		_, err := h.uc.CreatePayment(bg, "order", &domain.RequestCreatePayment{ReferenceID: "ORD-1", Amount: 99})
		assert.Equal(t, 409, appErrStatus(err), "same reference, different amount")

		other, err := h.uc.CreatePayment(bg, "kitchen", &domain.RequestCreatePayment{ReferenceID: "ORD-1", Amount: 50000})
		require.NoError(t, err)
		assert.NotEqual(t, first.PaymentID, other.PaymentID, "another source has its own namespace")
	})

	t.Run("an overdue open payment is expired and replaced", func(t *testing.T) {
		h := newHarness(t)
		old := h.create(t, "ORD-1", 50000)
		h.now = h.now.Add(48 * time.Hour)

		fresh := h.create(t, "ORD-1", 50000)
		assert.NotEqual(t, old.PaymentID, fresh.PaymentID)
		assert.Equal(t, shareddomain.PaymentExpired, h.repo.pay.payments[old.PaymentID].Status)
		assert.Len(t, h.repo.pay.events("payment.expired"), 1)
	})

	t.Run("a paid payment does not block a new one for the same reference", func(t *testing.T) {
		h := newHarness(t)
		first := h.create(t, "ORD-1", 50000)
		_, err := h.uc.Pay(bg, first.Token, "cash")
		require.NoError(t, err)
		cash, err := h.uc.GetCashPayment(bg, "CASH0002")
		require.NoError(t, err)
		_, err = h.uc.ConfirmCashPayment(bg, cash.CashCode, "cashier-1", &domain.RequestConfirmCash{AmountReceived: 50000})
		require.NoError(t, err)

		second := h.create(t, "ORD-1", 50000)
		assert.NotEqual(t, first.PaymentID, second.PaymentID)
	})

	t.Run("validation", func(t *testing.T) {
		h := newHarness(t)
		cases := map[string]struct {
			source string
			req    domain.RequestCreatePayment
		}{
			"no source":        {"", domain.RequestCreatePayment{ReferenceID: "r", Amount: 1}},
			"no reference":     {"order", domain.RequestCreatePayment{Amount: 1}},
			"zero amount":      {"order", domain.RequestCreatePayment{ReferenceID: "r"}},
			"foreign currency": {"order", domain.RequestCreatePayment{ReferenceID: "r", Amount: 1, Currency: "USD"}},
			"bad email":        {"order", domain.RequestCreatePayment{ReferenceID: "r", Amount: 1, Customer: shareddomain.Customer{Email: "nope"}}},
			"expiry too short": {"order", domain.RequestCreatePayment{ReferenceID: "r", Amount: 1, ExpiresInSec: 5}},
			"bad item":         {"order", domain.RequestCreatePayment{ReferenceID: "r", Amount: 1, Items: []shareddomain.Item{{Name: "x", Quantity: 0}}}},
		}
		for name, c := range cases {
			req := c.req
			_, err := h.uc.CreatePayment(bg, c.source, &req)
			assert.Equal(t, 400, appErrStatus(err), name)
		}
		assert.Empty(t, h.repo.pay.payments)
	})

	t.Run("custom expiry", func(t *testing.T) {
		h := newHarness(t)
		res, err := h.uc.CreatePayment(bg, "order", &domain.RequestCreatePayment{ReferenceID: "r", Amount: 1000, ExpiresInSec: 600})
		require.NoError(t, err)
		assert.Equal(t, h.now.Add(10*time.Minute).Format(time.RFC3339), res.ExpiresAt)
	})
}

func Test_CheckoutMethods(t *testing.T) {
	h := newHarness(t)
	res := h.create(t, "ORD-1", 100000)

	methods, err := h.uc.GetCheckoutMethods(bg, res.Token)
	require.NoError(t, err)
	var codes []string
	for _, m := range methods {
		codes = append(codes, m.Code)
	}
	// xendit's gateway is disabled and old_method is switched off
	assert.Equal(t, []string{"cash", "bca_va", "qris"}, codes)
	assert.EqualValues(t, 101000, methods[1].TotalAmount, "flat fee")
	assert.EqualValues(t, 700, methods[2].Fee, "0.7% of 100000")

	t.Run("amount range", func(t *testing.T) {
		low := h.create(t, "ORD-LOW", 500)
		ms, err := h.uc.GetCheckoutMethods(bg, low.Token)
		require.NoError(t, err)
		for _, m := range ms {
			assert.NotEqual(t, "qris", m.Code, "below qris minimum")
		}
	})

	t.Run("allowed methods restrict the list", func(t *testing.T) {
		p, err := h.uc.CreatePayment(bg, "order", &domain.RequestCreatePayment{ReferenceID: "ORD-2", Amount: 100000, AllowedMethods: []string{"cash"}})
		require.NoError(t, err)
		ms, err := h.uc.GetCheckoutMethods(bg, p.Token)
		require.NoError(t, err)
		require.Len(t, ms, 1)
		assert.Equal(t, "cash", ms[0].Code)
	})

	t.Run("a gateway without credentials hides its methods", func(t *testing.T) {
		g := h.repo.gateway.gateways["midtrans"]
		g.CredentialsEnc = ""
		h.repo.gateway.gateways["midtrans"] = g
		ms, err := h.uc.GetCheckoutMethods(bg, res.Token)
		require.NoError(t, err)
		require.Len(t, ms, 1)
		assert.Equal(t, "cash", ms[0].Code)
	})

	t.Run("unknown token", func(t *testing.T) {
		_, err := h.uc.GetCheckoutMethods(bg, "nope")
		assert.Equal(t, 404, appErrStatus(err))
	})
}

func Test_SelectMethod(t *testing.T) {
	h := newHarness(t)
	res := h.create(t, "ORD-1", 100000)

	co, err := h.uc.SelectMethod(bg, res.Token, "bca_va")
	require.NoError(t, err)
	assert.Equal(t, "bca_va", co.MethodCode)
	assert.EqualValues(t, 1000, co.Fee)
	assert.EqualValues(t, 101000, co.TotalAmount)
	assert.Equal(t, "pending", co.Status, "selecting does not start a checkout")
	assert.Empty(t, h.repo.pay.events("payment.checkout_started"))

	_, err = h.uc.SelectMethod(bg, res.Token, "xendit_invoice")
	assert.Equal(t, 400, appErrStatus(err), "method of a disabled gateway")
	_, err = h.uc.SelectMethod(bg, res.Token, "nope")
	assert.Equal(t, 400, appErrStatus(err))
}

func Test_Pay_Cash(t *testing.T) {
	h := newHarness(t)
	res := h.create(t, "ORD-1", 100000)

	co, err := h.uc.Pay(bg, res.Token, "cash")
	require.NoError(t, err)
	assert.Equal(t, "processing", co.Status)
	require.NotNil(t, co.Transaction)
	assert.Equal(t, "cash", co.Transaction.Instruction["type"])
	code, _ := co.Transaction.Instruction["cashCode"].(string)
	assert.NotEmpty(t, code)
	assert.Empty(t, h.prov.charges, "cash never touches a gateway")

	t.Run("every checkout is queued for Kafka", func(t *testing.T) {
		evs := h.repo.pay.events("payment.checkout_started")
		require.Len(t, evs, 1)
		ev := eventOf(t, evs[0])
		assert.Equal(t, co.Transaction.ID, ev.TransactionID)
		assert.Equal(t, "cash", ev.MethodCode)
		assert.Equal(t, "processing", ev.Status)
	})

	t.Run("checkout email", func(t *testing.T) {
		mails := h.repo.pay.events("notification_email")
		require.Len(t, mails, 1)
		var n domain.NotificationRequest
		require.NoError(t, json.Unmarshal(mails[0].Payload, &n))
		assert.Equal(t, "email", n.Channel)
		assert.Equal(t, "payment_checkout", n.TemplateCode)
		assert.Equal(t, "budi@example.com", n.Recipient)
		assert.Equal(t, "Rp100.000", n.Variables["totalAmount"])
		assert.Contains(t, n.Variables["instructionLines"], "Cash payment code: "+code)
	})

	t.Run("paying again for the running attempt creates nothing new", func(t *testing.T) {
		again, err := h.uc.Pay(bg, res.Token, "cash")
		require.NoError(t, err)
		assert.Equal(t, co.Transaction.ID, again.Transaction.ID)
		assert.Len(t, h.repo.pay.txns, 1)
		assert.Len(t, h.repo.pay.events("payment.checkout_started"), 1)
		assert.Len(t, h.repo.pay.events("notification_email"), 1)
	})
}

func Test_Pay_Gateway(t *testing.T) {
	t.Run("charges the gateway once with the fee included", func(t *testing.T) {
		h := newHarness(t)
		res := h.create(t, "ORD-1", 100000)

		co, err := h.uc.Pay(bg, res.Token, "bca_va")
		require.NoError(t, err)
		assert.Equal(t, "processing", co.Status)
		assert.EqualValues(t, 101000, co.TotalAmount)
		assert.Equal(t, "123456", co.Transaction.Instruction["vaNumber"])

		require.Len(t, h.prov.charges, 1)
		req := h.prov.charges[0]
		assert.Equal(t, co.Transaction.ID, req.TransactionID, "the transaction id is the gateway order id")
		assert.EqualValues(t, 101000, req.Amount)
		assert.Equal(t, "bca", req.Method.Channel)
		require.Len(t, req.Items, 2, "the fee is an item so the lines add up")
		assert.EqualValues(t, 1000, req.Items[1].Price)

		txn := h.repo.pay.txns[co.Transaction.ID]
		assert.Equal(t, "midtrans", txn.GatewayCode)
		assert.Equal(t, "ext-"+txn.ID, decodeMap(txn.GatewayResponse)["externalRef"])

		assert.Len(t, h.repo.pay.events("payment.checkout_started"), 1)
		mails := h.repo.pay.events("notification_email")
		require.Len(t, mails, 1)
		var n domain.NotificationRequest
		require.NoError(t, json.Unmarshal(mails[0].Payload, &n))
		assert.Contains(t, n.Variables["instructionLines"], "Virtual account number: 123456")

		again, err := h.uc.Pay(bg, res.Token, "bca_va")
		require.NoError(t, err)
		assert.Equal(t, co.Transaction.ID, again.Transaction.ID)
		assert.Len(t, h.prov.charges, 1, "no second charge for the same running attempt")
	})

	t.Run("a rejected charge still counts as a checkout and can be retried", func(t *testing.T) {
		h := newHarness(t)
		res := h.create(t, "ORD-1", 100000)
		h.prov.chargeErr = errors.New("gateway down")

		_, err := h.uc.Pay(bg, res.Token, "bca_va")
		assert.Equal(t, 502, appErrStatus(err))
		assert.NotContains(t, err.Error(), "gateway down", "gateway details are not shown to the customer")

		assert.Len(t, h.repo.pay.events("payment.checkout_started"), 1, "the attempt was queued before the gateway call")
		assert.Empty(t, h.repo.pay.events("notification_email"), "no instruction, no email")
		var txn shareddomain.Transaction
		for _, x := range h.repo.pay.txns {
			txn = x
		}
		assert.Equal(t, shareddomain.TransactionFailed, txn.Status)
		assert.Contains(t, txn.FailureReason, "gateway down")
		assert.Equal(t, shareddomain.PaymentPending, h.repo.pay.payments[res.PaymentID].Status)

		h.prov.chargeErr = nil
		co, err := h.uc.Pay(bg, res.Token, "bca_va")
		require.NoError(t, err)
		assert.Equal(t, "processing", co.Status)
		assert.Len(t, h.repo.pay.events("payment.checkout_started"), 2)
	})

	t.Run("a disabled gateway is reported as unavailable", func(t *testing.T) {
		h := newHarness(t)
		res := h.create(t, "ORD-1", 100000)
		g := h.repo.gateway.gateways["midtrans"]
		g.IsEnabled = false
		h.repo.gateway.gateways["midtrans"] = g
		_, err := h.uc.Pay(bg, res.Token, "bca_va")
		assert.Equal(t, 400, appErrStatus(err), "the method is not offered any more")
		assert.Empty(t, h.repo.pay.txns)
	})

	t.Run("switching method cancels the running attempt at its gateway", func(t *testing.T) {
		h := newHarness(t)
		res := h.create(t, "ORD-1", 100000)
		first, err := h.uc.Pay(bg, res.Token, "bca_va")
		require.NoError(t, err)

		second, err := h.uc.Pay(bg, res.Token, "cash")
		require.NoError(t, err)
		assert.NotEqual(t, first.Transaction.ID, second.Transaction.ID)
		assert.Equal(t, shareddomain.TransactionCancelled, h.repo.pay.txns[first.Transaction.ID].Status)
		assert.Equal(t, []string{first.Transaction.ID}, h.prov.cancelled)
		assert.EqualValues(t, 100000, second.TotalAmount, "the fee follows the new method")
		assert.Len(t, h.repo.pay.events("payment.checkout_started"), 2)
	})

	t.Run("no method selected", func(t *testing.T) {
		h := newHarness(t)
		res := h.create(t, "ORD-1", 100000)
		_, err := h.uc.Pay(bg, res.Token, "")
		assert.Equal(t, 400, appErrStatus(err))
	})

	t.Run("an expired link cannot be paid", func(t *testing.T) {
		h := newHarness(t)
		res := h.create(t, "ORD-1", 100000)
		h.now = h.now.Add(25 * time.Hour)
		_, err := h.uc.Pay(bg, res.Token, "cash")
		assert.Equal(t, 409, appErrStatus(err))
		assert.Equal(t, shareddomain.PaymentExpired, h.repo.pay.payments[res.PaymentID].Status)
	})

	t.Run("without a customer email no mail is queued but the checkout event is", func(t *testing.T) {
		h := newHarness(t)
		res, err := h.uc.CreatePayment(bg, "order", &domain.RequestCreatePayment{ReferenceID: "r", Amount: 10000})
		require.NoError(t, err)
		_, err = h.uc.Pay(bg, res.Token, "cash")
		require.NoError(t, err)
		assert.Empty(t, h.repo.pay.events("notification_email"))
		assert.Len(t, h.repo.pay.events("payment.checkout_started"), 1)
	})
}

// gatewayCallback makes the fake provider return a verified callback for a transaction
func (h *harness) gatewayCallback(txnID, status string, amount int64) *domain.CallbackMessage {
	h.prov.callback = provider.CallbackResult{TransactionID: txnID, Status: status, Amount: amount}
	h.prov.callbackErr = nil
	return &domain.CallbackMessage{Topic: "payment.midtrans_callback_received", Partition: 0, Offset: 7,
		Headers: map[string]string{"x-callback-token": "secret-token"}, Body: []byte(`{"raw":"body"}`)}
}

func Test_HandleCallback(t *testing.T) {
	setup := func(t *testing.T) (*harness, domain.ResponseCreatePayment, string) {
		h := newHarness(t)
		res := h.create(t, "ORD-1", 100000)
		co, err := h.uc.Pay(bg, res.Token, "bca_va")
		require.NoError(t, err)
		return h, res, co.Transaction.ID
	}

	t.Run("paid settles the payment and queues completion events", func(t *testing.T) {
		h, res, txnID := setup(t)
		out, err := h.uc.HandleCallback(bg, h.gatewayCallback(txnID, provider.CallbackPaid, 101000))
		require.NoError(t, err)
		assert.Equal(t, shareddomain.CallbackProcessed, out.Status)

		p := h.repo.pay.payments[res.PaymentID]
		assert.Equal(t, shareddomain.PaymentPaid, p.Status)
		require.NotNil(t, p.PaidAt)
		assert.EqualValues(t, 1000, p.Fee)
		assert.EqualValues(t, 101000, p.TotalAmount)
		assert.Equal(t, shareddomain.TransactionPaid, h.repo.pay.txns[txnID].Status)

		evs := h.repo.pay.events("payment.completed")
		require.Len(t, evs, 1)
		ev := eventOf(t, evs[0])
		assert.Equal(t, "paid", ev.Status)
		assert.Equal(t, txnID, ev.TransactionID)
		assert.Equal(t, "midtrans", ev.GatewayCode)

		mails := h.repo.pay.events("notification_email")
		require.Len(t, mails, 2, "checkout + paid")
		var n domain.NotificationRequest
		require.NoError(t, json.Unmarshal(mails[1].Payload, &n))
		assert.Equal(t, "payment_paid", n.TemplateCode)

		require.Len(t, h.repo.pay.logs, 1)
		assert.Equal(t, "processed", h.repo.pay.logs[0].Status)
		assert.Equal(t, "payment.midtrans_callback_received", h.repo.pay.logs[0].Topic)
		assert.Equal(t, txnID, h.repo.pay.logs[0].ExternalID)
		assert.True(t, isSealed(decodeStrMap(h.repo.pay.logs[0].Headers)["x-callback-token"]), "authenticating headers are stored encrypted")
	})

	t.Run("delivering the same callback twice changes nothing", func(t *testing.T) {
		h, _, txnID := setup(t)
		_, err := h.uc.HandleCallback(bg, h.gatewayCallback(txnID, provider.CallbackPaid, 101000))
		require.NoError(t, err)
		out, err := h.uc.HandleCallback(bg, h.gatewayCallback(txnID, provider.CallbackPaid, 101000))
		require.NoError(t, err)
		assert.Equal(t, shareddomain.CallbackDuplicate, out.Status)
		assert.Len(t, h.repo.pay.events("payment.completed"), 1)
	})

	t.Run("an amount that does not match is rejected without changing state", func(t *testing.T) {
		h, res, txnID := setup(t)
		out, err := h.uc.HandleCallback(bg, h.gatewayCallback(txnID, provider.CallbackPaid, 1))
		require.NoError(t, err)
		assert.Equal(t, shareddomain.CallbackFailed, out.Status)
		assert.Contains(t, out.Message, "amount mismatch")
		assert.Equal(t, shareddomain.PaymentProcessing, h.repo.pay.payments[res.PaymentID].Status)
		assert.Empty(t, h.repo.pay.events("payment.completed"))
	})

	t.Run("a failed callback frees the payment for another method", func(t *testing.T) {
		h, res, txnID := setup(t)
		out, err := h.uc.HandleCallback(bg, h.gatewayCallback(txnID, provider.CallbackFailed, 101000))
		require.NoError(t, err)
		assert.Equal(t, shareddomain.CallbackProcessed, out.Status)
		assert.Equal(t, shareddomain.TransactionFailed, h.repo.pay.txns[txnID].Status)
		assert.Equal(t, shareddomain.PaymentPending, h.repo.pay.payments[res.PaymentID].Status)

		_, err = h.uc.Pay(bg, res.Token, "cash")
		assert.NoError(t, err)
	})

	t.Run("an expired callback expires only the attempt", func(t *testing.T) {
		h, res, txnID := setup(t)
		_, err := h.uc.HandleCallback(bg, h.gatewayCallback(txnID, provider.CallbackExpired, 101000))
		require.NoError(t, err)
		assert.Equal(t, shareddomain.TransactionExpired, h.repo.pay.txns[txnID].Status)
		assert.Equal(t, shareddomain.PaymentPending, h.repo.pay.payments[res.PaymentID].Status)
	})

	t.Run("a late failure never undoes a paid payment", func(t *testing.T) {
		h, res, txnID := setup(t)
		_, _ = h.uc.HandleCallback(bg, h.gatewayCallback(txnID, provider.CallbackPaid, 101000))
		out, err := h.uc.HandleCallback(bg, h.gatewayCallback(txnID, provider.CallbackFailed, 101000))
		require.NoError(t, err)
		assert.Equal(t, shareddomain.CallbackIgnored, out.Status)
		assert.Equal(t, shareddomain.PaymentPaid, h.repo.pay.payments[res.PaymentID].Status)
	})

	t.Run("paying an attempt the customer already replaced still settles the payment", func(t *testing.T) {
		h, res, txnID := setup(t)
		_, err := h.uc.Pay(bg, res.Token, "cash") // replaces the bca attempt
		require.NoError(t, err)
		out, err := h.uc.HandleCallback(bg, h.gatewayCallback(txnID, provider.CallbackPaid, 101000))
		require.NoError(t, err)
		assert.Equal(t, shareddomain.CallbackProcessed, out.Status)
		p := h.repo.pay.payments[res.PaymentID]
		assert.Equal(t, shareddomain.PaymentPaid, p.Status)
		assert.Equal(t, "bca_va", p.MethodCode, "the payment takes the method that was really paid")
		open, ok := h.uc.openTransaction(bg, res.PaymentID)
		assert.False(t, ok, "the replacement attempt is closed, found %v", open.ID)
	})

	t.Run("money received for a cancelled payment is flagged for refund", func(t *testing.T) {
		h, res, txnID := setup(t)
		_, err := h.uc.CancelPayment(bg, res.PaymentID)
		require.NoError(t, err)
		out, err := h.uc.HandleCallback(bg, h.gatewayCallback(txnID, provider.CallbackPaid, 101000))
		require.NoError(t, err)
		assert.Equal(t, shareddomain.CallbackIgnored, out.Status)
		assert.Contains(t, out.Message, "refund required")
		assert.Equal(t, shareddomain.PaymentCancelled, h.repo.pay.payments[res.PaymentID].Status)
		assert.Equal(t, shareddomain.TransactionPaid, h.repo.pay.txns[txnID].Status, "recorded for reconciliation")
		assert.Empty(t, h.repo.pay.events("payment.completed"))
	})

	t.Run("a second payment of a paid payment is flagged for refund", func(t *testing.T) {
		h := newHarness(t)
		res := h.create(t, "ORD-1", 100000)
		a, _ := h.uc.Pay(bg, res.Token, "bca_va")
		_, err := h.uc.HandleCallback(bg, h.gatewayCallback(a.Transaction.ID, provider.CallbackPaid, 101000))
		require.NoError(t, err)

		// a stray second attempt of the same payment (e.g. paid via two tabs)
		stray := shareddomain.Transaction{ID: newUUID(), PaymentID: res.PaymentID, MethodCode: "bca_va", GatewayCode: "midtrans",
			Status: shareddomain.TransactionPending, Amount: 101000, ExpiresAt: h.now.Add(time.Hour)}
		require.NoError(t, h.repo.pay.SaveTransaction(bg, &stray))
		out, err := h.uc.HandleCallback(bg, h.gatewayCallback(stray.ID, provider.CallbackPaid, 101000))
		require.NoError(t, err)
		assert.Equal(t, shareddomain.CallbackIgnored, out.Status)
		assert.Contains(t, out.Message, "refund required")
		assert.Len(t, h.repo.pay.events("payment.completed"), 1)
	})

	t.Run("permanent problems are logged and not retried", func(t *testing.T) {
		h, _, txnID := setup(t)

		h.prov.callbackErr = provider.ErrInvalidCallback
		out, err := h.uc.HandleCallback(bg, &domain.CallbackMessage{Topic: "payment.midtrans_callback_received", Body: []byte("x")})
		assert.NoError(t, err, "a forged callback must not be retried")
		assert.Equal(t, shareddomain.CallbackFailed, out.Status)

		out, err = h.uc.HandleCallback(bg, &domain.CallbackMessage{Topic: "some.other.topic"})
		assert.NoError(t, err)
		assert.Equal(t, shareddomain.CallbackFailed, out.Status)
		assert.Contains(t, out.Message, "not a configured consume topic")

		out, err = h.uc.HandleCallback(bg, h.gatewayCallback(newUUID(), provider.CallbackPaid, 1))
		assert.NoError(t, err)
		assert.Equal(t, shareddomain.CallbackIgnored, out.Status, "unknown transaction")

		out, err = h.uc.HandleCallback(bg, h.gatewayCallback("not-a-uuid", provider.CallbackPaid, 1))
		assert.NoError(t, err)
		assert.Equal(t, shareddomain.CallbackIgnored, out.Status)

		// a callback of another gateway's topic cannot settle this transaction
		msg := h.gatewayCallback(txnID, provider.CallbackPaid, 101000)
		msg.Topic = "payment.xendit_callback_received"
		out, err = h.uc.HandleCallback(bg, msg)
		assert.NoError(t, err)
		assert.Equal(t, shareddomain.CallbackFailed, out.Status)
		assert.Contains(t, out.Message, "belongs to gateway")

		assert.Len(t, h.repo.pay.logs, 5, "everything is logged")
		assert.Empty(t, h.repo.pay.events("payment.completed"))
	})

	t.Run("pending notifications are ignored", func(t *testing.T) {
		h, _, txnID := setup(t)
		out, err := h.uc.HandleCallback(bg, h.gatewayCallback(txnID, provider.CallbackIgnored, 0))
		require.NoError(t, err)
		assert.Equal(t, shareddomain.CallbackIgnored, out.Status)
	})

	t.Run("a callback still settles after its gateway was disabled", func(t *testing.T) {
		h, res, txnID := setup(t)
		g := h.repo.gateway.gateways["midtrans"]
		g.IsEnabled = false
		h.repo.gateway.gateways["midtrans"] = g
		out, err := h.uc.HandleCallback(bg, h.gatewayCallback(txnID, provider.CallbackPaid, 101000))
		require.NoError(t, err)
		assert.Equal(t, shareddomain.CallbackProcessed, out.Status)
		assert.Equal(t, shareddomain.PaymentPaid, h.repo.pay.payments[res.PaymentID].Status)
	})

	t.Run("replay", func(t *testing.T) {
		h, res, txnID := setup(t)
		// first delivery fails permanently (forged/broken), then the operator fixes it and replays
		h.prov.callbackErr = provider.ErrInvalidCallback
		_, _ = h.uc.HandleCallback(bg, &domain.CallbackMessage{Topic: "payment.midtrans_callback_received", Offset: 1, Body: []byte("x")})
		require.Len(t, h.repo.pay.logs, 1)

		h.prov.callbackErr = nil
		h.prov.callback = provider.CallbackResult{TransactionID: txnID, Status: provider.CallbackPaid, Amount: 101000}
		out, err := h.uc.ReplayCallback(bg, h.repo.pay.logs[0].ID)
		require.NoError(t, err)
		assert.Equal(t, shareddomain.CallbackProcessed, out.Status)
		assert.Equal(t, shareddomain.PaymentPaid, h.repo.pay.payments[res.PaymentID].Status)

		_, err = h.uc.ReplayCallback(bg, h.repo.pay.logs[len(h.repo.pay.logs)-1].ID)
		assert.Equal(t, 409, appErrStatus(err), "a processed callback is not replayed")
		_, err = h.uc.ReplayCallback(bg, 999)
		assert.Equal(t, 404, appErrStatus(err))
	})

	t.Run("RecordCallbackFailure keeps a message that could not be processed", func(t *testing.T) {
		h := newHarness(t)
		h.uc.RecordCallbackFailure(bg, &domain.CallbackMessage{Topic: "payment.midtrans_callback_received", Offset: 9, Body: []byte("{}")}, errors.New("db down"))
		require.Len(t, h.repo.pay.logs, 1)
		assert.Equal(t, "failed", h.repo.pay.logs[0].Status)
		assert.Equal(t, "midtrans", h.repo.pay.logs[0].GatewayCode)
		assert.Contains(t, h.repo.pay.logs[0].Error, "db down")
	})
}

func decodeStrMap(j shareddomain.JSON) map[string]string {
	m := map[string]string{}
	_ = j.Decode(&m)
	return m
}

func Test_Cash(t *testing.T) {
	setup := func(t *testing.T) (*harness, domain.ResponseCreatePayment, string) {
		h := newHarness(t)
		res := h.create(t, "ORD-1", 100000)
		co, err := h.uc.Pay(bg, res.Token, "cash")
		require.NoError(t, err)
		return h, res, co.Transaction.Instruction["cashCode"].(string)
	}

	t.Run("lookup is case-insensitive", func(t *testing.T) {
		h, res, code := setup(t)
		got, err := h.uc.GetCashPayment(bg, " "+strings.ToLower(code)+" ")
		require.NoError(t, err)
		assert.Equal(t, res.PaymentID, got.PaymentID)
		assert.EqualValues(t, 100000, got.TotalAmount)
		_, err = h.uc.GetCashPayment(bg, "NOPE")
		assert.Equal(t, 404, appErrStatus(err))
	})

	t.Run("confirm reports change and settles the payment", func(t *testing.T) {
		h, res, code := setup(t)
		out, err := h.uc.ConfirmCashPayment(bg, code, "cashier-7", &domain.RequestConfirmCash{AmountReceived: 120000})
		require.NoError(t, err)
		assert.EqualValues(t, 20000, out.Change)
		assert.Equal(t, "paid", out.Status)

		assert.Equal(t, shareddomain.PaymentPaid, h.repo.pay.payments[res.PaymentID].Status)
		var txn shareddomain.Transaction
		for _, x := range h.repo.pay.txns {
			txn = x
		}
		assert.Equal(t, "cashier-7", *txn.ConfirmedBy)
		assert.EqualValues(t, 120000, *txn.CashReceived)
		assert.Len(t, h.repo.pay.events("payment.completed"), 1)
		assert.Len(t, h.repo.pay.events("notification_email"), 2)

		_, err = h.uc.ConfirmCashPayment(bg, code, "cashier-7", &domain.RequestConfirmCash{AmountReceived: 100000})
		assert.Equal(t, 409, appErrStatus(err), "cannot confirm twice")
		assert.Len(t, h.repo.pay.events("payment.completed"), 1)
	})

	t.Run("too little cash", func(t *testing.T) {
		h, res, code := setup(t)
		_, err := h.uc.ConfirmCashPayment(bg, code, "c", &domain.RequestConfirmCash{AmountReceived: 99999})
		assert.Equal(t, 400, appErrStatus(err))
		assert.Equal(t, shareddomain.PaymentProcessing, h.repo.pay.payments[res.PaymentID].Status)
	})

	t.Run("an expired or cancelled payment cannot be confirmed", func(t *testing.T) {
		h, _, code := setup(t)
		h.now = h.now.Add(25 * time.Hour)
		_, err := h.uc.ConfirmCashPayment(bg, code, "c", &domain.RequestConfirmCash{AmountReceived: 100000})
		assert.Equal(t, 409, appErrStatus(err))

		h2, res2, code2 := setup(t)
		_, err = h2.uc.CancelPayment(bg, res2.PaymentID)
		require.NoError(t, err)
		_, err = h2.uc.ConfirmCashPayment(bg, code2, "c", &domain.RequestConfirmCash{AmountReceived: 100000})
		assert.Equal(t, 409, appErrStatus(err))
	})
}

func Test_Cancel(t *testing.T) {
	t.Run("caller cancels an open payment, twice is fine", func(t *testing.T) {
		h := newHarness(t)
		res := h.create(t, "ORD-1", 100000)
		co, err := h.uc.Pay(bg, res.Token, "bca_va")
		require.NoError(t, err)

		out, err := h.uc.CancelPayment(bg, res.PaymentID)
		require.NoError(t, err)
		assert.Equal(t, "cancelled", out.Status)
		assert.Equal(t, shareddomain.TransactionCancelled, h.repo.pay.txns[co.Transaction.ID].Status)
		assert.Equal(t, []string{co.Transaction.ID}, h.prov.cancelled, "the charge is cancelled at the gateway")
		assert.Len(t, h.repo.pay.events("payment.cancelled"), 1)

		_, err = h.uc.CancelPayment(bg, res.PaymentID)
		require.NoError(t, err)
		assert.Len(t, h.repo.pay.events("payment.cancelled"), 1, "no duplicate event")

		_, err = h.uc.Pay(bg, res.Token, "cash")
		assert.Equal(t, 409, appErrStatus(err))
	})

	t.Run("paid or expired payments cannot be cancelled", func(t *testing.T) {
		h := newHarness(t)
		res := h.create(t, "ORD-1", 100000)
		co, _ := h.uc.Pay(bg, res.Token, "cash")
		_, err := h.uc.ConfirmCashPayment(bg, co.Transaction.Instruction["cashCode"].(string), "c", &domain.RequestConfirmCash{AmountReceived: 100000})
		require.NoError(t, err)
		_, err = h.uc.CancelPayment(bg, res.PaymentID)
		assert.Equal(t, 409, appErrStatus(err))

		_, err = h.uc.CancelPayment(bg, "not-a-uuid")
		assert.Equal(t, 404, appErrStatus(err))
		_, err = h.uc.CancelPayment(bg, newUUID())
		assert.Equal(t, 404, appErrStatus(err))
	})

	t.Run("the customer can cancel from checkout", func(t *testing.T) {
		h := newHarness(t)
		res := h.create(t, "ORD-1", 100000)
		co, err := h.uc.CancelCheckout(bg, res.Token)
		require.NoError(t, err)
		assert.Equal(t, "cancelled", co.Status)
		assert.Len(t, h.repo.pay.events("payment.cancelled"), 1)
	})
}

func Test_ExpireOverduePayments(t *testing.T) {
	h := newHarness(t)
	open := h.create(t, "ORD-OPEN", 100000)
	overdue := h.create(t, "ORD-OVERDUE", 100000)
	paid := h.create(t, "ORD-PAID", 100000)

	co, err := h.uc.Pay(bg, overdue.Token, "bca_va")
	require.NoError(t, err)
	pc, _ := h.uc.Pay(bg, paid.Token, "cash")
	_, err = h.uc.ConfirmCashPayment(bg, pc.Transaction.Instruction["cashCode"].(string), "c", &domain.RequestConfirmCash{AmountReceived: 100000})
	require.NoError(t, err)

	// only ORD-OVERDUE and ORD-PAID are old; move them back in time
	for _, id := range []string{overdue.PaymentID, paid.PaymentID} {
		p := h.repo.pay.payments[id]
		p.ExpiresAt = h.now.Add(-time.Minute)
		h.repo.pay.payments[id] = p
	}

	n, err := h.uc.ExpireOverduePayments(bg)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, shareddomain.PaymentExpired, h.repo.pay.payments[overdue.PaymentID].Status)
	assert.Equal(t, shareddomain.TransactionExpired, h.repo.pay.txns[co.Transaction.ID].Status)
	assert.Equal(t, shareddomain.PaymentPaid, h.repo.pay.payments[paid.PaymentID].Status, "paid payments never expire")
	assert.Equal(t, shareddomain.PaymentPending, h.repo.pay.payments[open.PaymentID].Status)
	assert.Equal(t, []string{co.Transaction.ID}, h.prov.cancelled, "the gateway charge is cancelled")

	evs := h.repo.pay.events("payment.expired")
	require.Len(t, evs, 1)
	assert.Equal(t, overdue.PaymentID, eventOf(t, evs[0]).PaymentID)

	n, err = h.uc.ExpireOverduePayments(bg)
	require.NoError(t, err)
	assert.Zero(t, n, "idempotent")
}

func Test_FlushOutbox(t *testing.T) {
	setup := func(t *testing.T) *harness {
		h := newHarness(t)
		res := h.create(t, "ORD-1", 100000)
		_, err := h.uc.Pay(bg, res.Token, "cash")
		require.NoError(t, err)
		return h
	}

	t.Run("publishes each event to its DB-configured topic", func(t *testing.T) {
		h := setup(t)
		n, err := h.uc.FlushOutbox(bg)
		require.NoError(t, err)
		assert.Equal(t, 3, n)
		assert.Equal(t, []string{"payment.created", "payment.checkout_started", "notification.requested"}, h.pub.topics())
		for _, m := range h.pub.msgs {
			assert.NotEmpty(t, m.Key)
			assert.Equal(t, "application/json", m.ContentType)
		}
		var pending int
		for _, o := range h.repo.pay.outbox {
			if o.PublishedAt == nil {
				pending++
			}
		}
		assert.Zero(t, pending)

		n, _ = h.uc.FlushOutbox(bg)
		assert.Zero(t, n, "nothing is published twice")
	})

	t.Run("the topic name is read from the DB at publish time", func(t *testing.T) {
		h := setup(t)
		for i := range h.repo.topic.topics {
			if h.repo.topic.topics[i].Topic == "payment.checkout_started" {
				h.repo.topic.topics[i].Topic = "custom.checkout"
			}
		}
		_, err := h.uc.FlushOutbox(bg)
		require.NoError(t, err)
		assert.Contains(t, h.pub.topics(), "custom.checkout")
	})

	t.Run("fan-out to several topics of one event", func(t *testing.T) {
		h := setup(t)
		h.repo.topic.topics = append(h.repo.topic.topics, shareddomain.Topic{ID: 99, Topic: "audit.checkout", Direction: "publish", EventType: strp("payment.checkout_started"), IsEnabled: true})
		_, err := h.uc.FlushOutbox(bg)
		require.NoError(t, err)
		assert.Contains(t, h.pub.topics(), "audit.checkout")
		assert.Contains(t, h.pub.topics(), "payment.checkout_started")
	})

	t.Run("an event without an enabled topic is dropped, not retried forever", func(t *testing.T) {
		h := setup(t)
		for i := range h.repo.topic.topics {
			if h.repo.topic.topics[i].Topic == "notification.requested" {
				h.repo.topic.topics[i].IsEnabled = false
			}
		}
		n, err := h.uc.FlushOutbox(bg)
		require.NoError(t, err)
		assert.Equal(t, 2, n)
		mails := h.repo.pay.events("notification_email")
		require.Len(t, mails, 1)
		assert.NotNil(t, mails[0].PublishedAt)
		assert.Contains(t, mails[0].LastError, "dropped")
	})

	t.Run("a Kafka outage keeps events for the next run and counts attempts", func(t *testing.T) {
		h := setup(t)
		h.pub.err = errors.New("kafka down")
		n, err := h.uc.FlushOutbox(bg)
		require.NoError(t, err)
		assert.Zero(t, n)
		first := h.repo.pay.outbox[0]
		assert.Nil(t, first.PublishedAt)
		assert.Equal(t, 1, first.Attempts)
		assert.Contains(t, first.LastError, "kafka down")
		assert.Zero(t, h.repo.pay.outbox[1].Attempts, "the batch stops at the first failure to keep order")

		h.pub.err = nil
		n, err = h.uc.FlushOutbox(bg)
		require.NoError(t, err)
		assert.Equal(t, 3, n)
		assert.Equal(t, "payment.created", h.pub.topics()[0], "order is preserved")
	})

	t.Run("a second flush while one is running is skipped", func(t *testing.T) {
		h := setup(t)
		h.uc.flushMu.Lock()
		n, err := h.uc.FlushOutbox(bg)
		h.uc.flushMu.Unlock()
		require.NoError(t, err)
		assert.Zero(t, n)
		assert.Empty(t, h.pub.msgs)
	})
}

func Test_RollbackKeepsStateAndOutboxConsistent(t *testing.T) {
	h := newHarness(t)
	res := h.create(t, "ORD-1", 100000)

	// a failing save inside the attempt transaction must leave no half-created checkout behind
	before := len(h.repo.pay.outbox)
	h.uc.newCashCode = func() string { return "DUPLICATE" }
	_, err := h.uc.Pay(bg, res.Token, "cash")
	require.NoError(t, err)

	other := h.create(t, "ORD-2", 100000)
	_, err = h.uc.Pay(bg, other.Token, "cash") // same cash code: unique violation
	require.Error(t, err)
	assert.Equal(t, shareddomain.PaymentPending, h.repo.pay.payments[other.PaymentID].Status, "payment untouched")
	for _, tx := range h.repo.pay.txns {
		assert.NotEqual(t, other.PaymentID, tx.PaymentID, "no orphan transaction")
	}
	assert.Equal(t, before+2+1, len(h.repo.pay.outbox), "only ORD-1 checkout (event+mail) and ORD-2's creation event exist; nothing from the failed attempt")
}

func Test_SimulateMockCallback(t *testing.T) {
	h := newHarness(t)
	res := h.create(t, "ORD-1", 100000)
	h.repo.method.methods = append(h.repo.method.methods, shareddomain.Method{ID: 9, Code: "mock_va", Name: "Mock", Type: "virtual_account", GatewayCode: strp("mock"), GatewayChannel: "va", IsEnabled: true})
	co, err := h.uc.Pay(bg, res.Token, "mock_va")
	require.NoError(t, err)

	require.NoError(t, h.uc.SimulateMockCallback(bg, &domain.RequestMockCallback{TransactionID: co.Transaction.ID, Status: "paid"}))
	require.Len(t, h.pub.msgs, 1)
	msg := h.pub.msgs[0]
	assert.Equal(t, "payment.mock_callback_received", msg.Topic)
	var env struct {
		Headers map[string]string `json:"headers"`
		Body    struct {
			TransactionID string `json:"transactionId"`
			Status        string `json:"status"`
			Amount        int64  `json:"amount"`
		} `json:"body"`
	}
	require.NoError(t, json.Unmarshal(msg.Message, &env))
	assert.Equal(t, co.Transaction.ID, env.Body.TransactionID)
	assert.Equal(t, "paid", env.Body.Status)
	assert.EqualValues(t, 100000, env.Body.Amount)

	t.Run("refused in production", func(t *testing.T) {
		h.env.Environment = "production"
		err := h.uc.SimulateMockCallback(bg, &domain.RequestMockCallback{TransactionID: co.Transaction.ID, Status: "paid"})
		assert.Equal(t, 403, appErrStatus(err))
	})
	t.Run("only mock transactions", func(t *testing.T) {
		h.env.Environment = ""
		cash, _ := h.uc.Pay(bg, res.Token, "cash")
		err := h.uc.SimulateMockCallback(bg, &domain.RequestMockCallback{TransactionID: cash.Transaction.ID, Status: "paid"})
		assert.Equal(t, 400, appErrStatus(err))
	})
}

func Test_ConsumeTopics(t *testing.T) {
	h := newHarness(t)
	topics, err := h.uc.ConsumeTopics(bg)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"payment.midtrans_callback_received", "payment.mock_callback_received"}, topics,
		"xendit's gateway is disabled, so its topic is not consumed")

	g := h.repo.gateway.gateways["xendit"]
	g.IsEnabled = true
	h.repo.gateway.gateways["xendit"] = g
	topics, _ = h.uc.ConsumeTopics(bg)
	assert.Len(t, topics, 3, "enabling a gateway adds its topic without a restart")
}
