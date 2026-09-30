package usecase

import (
	"testing"
	"time"

	"monorepo/services/order/internal/modules/order/domain"
	shareddomain "monorepo/services/order/pkg/shared/domain"

	"github.com/stretchr/testify/assert"
)

func Test_AllowedOrderStatuses(t *testing.T) {
	o := func(order, payment string) *shareddomain.Order {
		return &shareddomain.Order{OrderStatus: order, PaymentStatus: payment}
	}
	assert.Equal(t, []string{"cancelled"}, AllowedOrderStatuses(o("awaiting_payment", "pending")), "only a walk-out before payment")
	assert.Equal(t, []string{"preparing", "ready", "completed", "cancelled"}, AllowedOrderStatuses(o("confirmed", "paid")))
	assert.Equal(t, []string{"completed", "cancelled"}, AllowedOrderStatuses(o("ready", "paid")), "never backwards")
	assert.Equal(t, []string{"refunded"}, AllowedOrderStatuses(o("completed", "paid")))
	assert.Empty(t, AllowedOrderStatuses(o("completed", "pending")), "nothing to refund")
	assert.Empty(t, AllowedOrderStatuses(o("cancelled", "paid")))
	assert.Empty(t, AllowedOrderStatuses(o("refunded", "paid")))
}

func Test_applyPaymentEvent(t *testing.T) {
	t0 := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	ev := func(event, status string, at time.Time) *domain.PaymentEvent {
		return &domain.PaymentEvent{Event: event, Status: status, OccurredAt: at, TotalAmount: 10000, Fee: 0, MethodCode: "qris", TransactionID: "t1"}
	}

	t.Run("applies and counts attempts", func(t *testing.T) {
		o := &shareddomain.Order{PaymentStatus: "pending"}
		flag, ok := applyPaymentEvent(o, ev("payment.checkout_started", "processing", t0))
		assert.True(t, ok)
		assert.Empty(t, flag)
		assert.Equal(t, "processing", o.PaymentStatus)
		assert.Equal(t, 1, o.AttemptCount)
		assert.Equal(t, "qris", o.MethodCode)
		assert.True(t, o.LastEventAt.Equal(t0))
	})

	t.Run("an older non-final event is stale", func(t *testing.T) {
		last := t0.Add(time.Minute)
		o := &shareddomain.Order{PaymentStatus: "processing", LastEventAt: &last}
		flag, ok := applyPaymentEvent(o, ev("payment.created", "pending", t0))
		assert.False(t, ok)
		assert.Equal(t, shareddomain.FlagStale, flag)
		assert.Equal(t, "processing", o.PaymentStatus)
	})

	t.Run("a final event applies even when older", func(t *testing.T) {
		last := t0.Add(time.Minute)
		o := &shareddomain.Order{PaymentStatus: "processing", LastEventAt: &last}
		_, ok := applyPaymentEvent(o, ev("payment.completed", "paid", t0))
		assert.True(t, ok)
		assert.Equal(t, "paid", o.PaymentStatus)
		assert.True(t, o.LastEventAt.Equal(last), "last event time never goes back")
	})

	t.Run("nothing after a final status", func(t *testing.T) {
		o := &shareddomain.Order{PaymentStatus: "paid", PaymentStatusSource: "kafka"}
		flag, ok := applyPaymentEvent(o, ev("payment.checkout_started", "processing", t0))
		assert.False(t, ok)
		assert.Equal(t, shareddomain.FlagStale, flag)
	})

	t.Run("a manual final status is not overwritten by Kafka", func(t *testing.T) {
		o := &shareddomain.Order{PaymentStatus: "cancelled", PaymentStatusSource: "manual"}
		flag, ok := applyPaymentEvent(o, ev("payment.completed", "paid", t0))
		assert.False(t, ok)
		assert.Equal(t, shareddomain.FlagIgnoredManual, flag)
		assert.Equal(t, "cancelled", o.PaymentStatus)
	})

	t.Run("a manual non-final status is overwritten by a later payment event", func(t *testing.T) {
		o := &shareddomain.Order{PaymentStatus: "pending", PaymentStatusSource: "manual"}
		_, ok := applyPaymentEvent(o, ev("payment.completed", "paid", t0))
		assert.True(t, ok)
		assert.Equal(t, "paid", o.PaymentStatus)
		assert.Equal(t, "kafka", o.PaymentStatusSource)
	})
}
