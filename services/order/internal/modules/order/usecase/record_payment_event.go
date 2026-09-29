package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"monorepo/globalshared/money"
	"monorepo/services/order/internal/modules/order/domain"
	shareddomain "monorepo/services/order/pkg/shared/domain"
	"monorepo/services/order/pkg/shared/usecase/common"

	"github.com/golangid/candi/tracer"
	"gorm.io/gorm"
)

// ErrBadEvent marks a payload that can never be booked; the consumer logs and skips it
var ErrBadEvent = errors.New("order: bad payment event")

var knownPaymentEvents = map[string]bool{
	shareddomain.EventPaymentCreated: true, shareddomain.EventPaymentCheckoutStarted: true,
	shareddomain.EventPaymentCompleted: true, shareddomain.EventPaymentExpired: true, shareddomain.EventPaymentCancelled: true,
}

var knownPaymentStatuses = map[string]bool{
	shareddomain.PaymentPending: true, shareddomain.PaymentProcessing: true, shareddomain.PaymentPaid: true,
	shareddomain.PaymentExpired: true, shareddomain.PaymentCancelled: true,
}

func (uc *orderUsecaseImpl) RecordPaymentEvent(ctx context.Context, ev *domain.PaymentEvent, src domain.KafkaSource) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "OrderUsecase:RecordPaymentEvent")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	if ev.PaymentID == "" || !knownPaymentEvents[ev.Event] || !knownPaymentStatuses[ev.Status] {
		return fmt.Errorf("%w: event %q, payment %q, status %q", ErrBadEvent, ev.Event, ev.PaymentID, ev.Status)
	}
	if ev.OccurredAt.IsZero() { // events of a payment service older than the snapshot payload
		ev.OccurredAt = uc.now()
	}
	trace.SetTag("payment_id", ev.PaymentID)
	trace.SetTag("event", ev.Event)

	var enqueued bool
	err = uc.repoSQL.WithTransaction(ctx, func(ctx context.Context) error {
		repo := uc.repoSQL.OrderRepo()
		// every event of a payment is booked one at a time, whatever topic/partition it came from
		if err := repo.LockPayment(ctx, ev.PaymentID); err != nil {
			return err
		}

		order, err := repo.FindByPaymentID(ctx, ev.PaymentID)
		created := false
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			if order, err = uc.createOrder(ctx, ev); err != nil {
				return err
			}
			created, enqueued = true, true
		case err != nil:
			return err
		default:
			dup, err := repo.EventExists(ctx, order.ID, ev.Event, ev.TransactionID)
			if err != nil || dup {
				return err // a redelivery: already booked
			}
		}

		entry := &shareddomain.OrderEvent{
			OrderID: order.ID, Event: ev.Event, Source: shareddomain.SourceKafka, TransactionID: ev.TransactionID,
			FromStatus: order.PaymentStatus, ToStatus: ev.Status, Amount: ev.Amount, Fee: ev.Fee,
			TotalAmount: ev.TotalAmount, MethodCode: ev.MethodCode, Payload: shareddomain.NewJSONValue(ev),
			Topic: src.Topic, Partition: src.Partition, Offset: src.Offset, OccurredAt: ev.OccurredAt,
		}

		prevPayment := order.PaymentStatus
		flag, applied := applyPaymentEvent(&order, ev)
		entry.Flag = flag
		if applied {
			auto, err := uc.afterPaymentChange(ctx, &order, prevPayment, shareddomain.SourceKafka, "")
			if err != nil {
				return err
			}
			if auto.flag != "" {
				entry.Flag = auto.flag
			}
			enqueued = enqueued || auto.enqueued
		}
		if _, err = repo.InsertEvent(ctx, entry); err != nil {
			return err
		}
		enqueued = true
		if err = uc.LogActivity(ctx, shareddomain.OrderActivity(ev.Event, &order, "", paymentActivityMessage(ev, entry.Flag),
			map[string]any{"transactionId": ev.TransactionID, "methodCode": ev.MethodCode, "totalAmount": ev.TotalAmount, "flag": entry.Flag})); err != nil {
			return err
		}
		if !applied && !created {
			return nil
		}
		if !created {
			order.Version++
		}
		return repo.Update(ctx, &order)
	})
	if err == nil && enqueued {
		uc.afterCommit()
	}
	return err
}

// createOrder books the first event seen of a payment: the order number, the items and the
// priced breakdown with the merchant's settings of today, then publishes order.created
func (uc *orderUsecaseImpl) createOrder(ctx context.Context, ev *domain.PaymentEvent) (order shareddomain.Order, err error) {
	merchantID := ev.Meta("merchantId")
	if merchantID == "" {
		merchantID = shareddomain.DefaultMerchant
	}
	m, err := uc.sharedUsecase.MerchantSettings(ctx, merchantID)
	if err != nil {
		return order, err
	}

	lines := make([]shareddomain.PriceLine, 0, len(ev.Items))
	for _, it := range ev.Items {
		lines = append(lines, shareddomain.PriceLine{Name: it.Name, Price: it.Price, Quantity: it.Quantity})
	}
	if len(lines) == 0 { // a payment without items is booked as one line of its amount
		name := strings.TrimSpace(ev.Description)
		if name == "" {
			name = "Payment " + ev.ReferenceID
		}
		lines = append(lines, shareddomain.PriceLine{Name: name, Price: ev.Amount, Quantity: 1})
	}
	b := common.Price(lines, m)

	placedAt := ev.CreatedAt
	if placedAt.IsZero() {
		placedAt = ev.OccurredAt
	}
	number, err := uc.sharedUsecase.NextOrderNumber(ctx, m, placedAt)
	if err != nil {
		return order, err
	}
	currency := ev.Currency
	if currency == "" {
		currency = "IDR"
	}

	order = shareddomain.Order{
		OrderNumber: number, PaymentID: ev.PaymentID, Source: ev.Source, ReferenceID: ev.ReferenceID,
		MerchantID: merchantID, OutletID: ev.Meta("outletId"), CashierID: ev.Meta("cashierId"), Channel: ev.Meta("channel"),
		Description: ev.Description, CustomerName: ev.Customer.Name, CustomerEmail: ev.Customer.Email, CustomerPhone: ev.Customer.Phone,
		Currency: currency, Subtotal: b.Subtotal, TaxAmount: b.TaxAmount, RoundingAdjustment: b.RoundingAdjustment,
		ExpectedTotal: b.Total, Amount: ev.Amount, Fee: ev.Fee, TotalAmount: ev.TotalAmount, AmountMismatch: ev.Amount != b.Total,
		TaxName: m.TaxName, TaxRate: m.TaxRate, TaxMode: b.TaxMode, RoundingMode: m.RoundingMode, RoundingUnit: m.RoundingUnit,
		PaymentStatus: shareddomain.PaymentPending, PaymentStatusSource: shareddomain.SourceKafka,
		OrderStatus: shareddomain.OrderAwaitingPayment, Metadata: shareddomain.NewJSONValue(ev.Metadata),
		PlacedAt: placedAt.UTC(), ExpiresAt: ev.ExpiresAt, Version: 1, Items: b.Lines,
	}
	if order.TotalAmount == 0 {
		order.TotalAmount = order.Amount + order.Fee
	}
	if err = uc.repoSQL.OrderRepo().Create(ctx, &order); err != nil {
		return order, err
	}
	if err = uc.Enqueue(ctx, shareddomain.EventOrderCreated, order.OrderNumber, orderEvent(shareddomain.EventOrderCreated, &order, placedAt, "", "")); err != nil {
		return order, err
	}
	msg := fmt.Sprintf("Order %s placed for %s (%s, reference %s)", order.OrderNumber, money.FormatIDR(order.Amount), order.Source, order.ReferenceID)
	if order.AmountMismatch {
		msg += fmt.Sprintf("; payment amount differs from the priced total %s", money.FormatIDR(order.ExpectedTotal))
	}
	return order, uc.LogActivity(ctx, shareddomain.OrderActivity(shareddomain.EventOrderCreated, &order, "", msg, nil))
}

func paymentActivityMessage(ev *domain.PaymentEvent, flag string) string {
	msg := fmt.Sprintf("Payment %s (%s)", ev.Status, ev.Event)
	if ev.MethodCode != "" {
		msg += " via " + ev.MethodCode
	}
	switch flag {
	case shareddomain.FlagIgnoredManual:
		msg += "; not applied, the payment status was set by hand"
	case shareddomain.FlagStale:
		msg += "; not applied, older than the current state"
	case shareddomain.FlagNeedsRefund:
		msg += "; the order was already cancelled, a refund is needed"
	}
	return msg
}

// applyPaymentEvent updates the payment side of an order from an event. It does not apply
//   - anything after an admin set a final payment status by hand (flag ignored_manual),
//   - anything after a final payment status, which the payment service never leaves (flag stale),
//   - a non-final status older than the last applied event (flag stale).
func applyPaymentEvent(o *shareddomain.Order, ev *domain.PaymentEvent) (flag string, applied bool) {
	if shareddomain.IsFinalPayment(o.PaymentStatus) {
		if o.PaymentStatusSource == shareddomain.SourceManual {
			return shareddomain.FlagIgnoredManual, false
		}
		return shareddomain.FlagStale, false
	}
	if !shareddomain.IsFinalPayment(ev.Status) && o.LastEventAt != nil && ev.OccurredAt.Before(*o.LastEventAt) {
		return shareddomain.FlagStale, false
	}

	o.PaymentStatus, o.PaymentStatusSource = ev.Status, shareddomain.SourceKafka
	if ev.TotalAmount > 0 {
		o.Fee, o.TotalAmount = ev.Fee, ev.TotalAmount
	}
	if ev.MethodCode != "" {
		o.MethodCode = ev.MethodCode
	}
	if ev.GatewayCode != "" {
		o.GatewayCode = ev.GatewayCode
	}
	if ev.TransactionID != "" {
		o.TransactionID = ev.TransactionID
	}
	if ev.Event == shareddomain.EventPaymentCheckoutStarted {
		o.AttemptCount++
	}
	if ev.PaidAt != nil {
		t := ev.PaidAt.UTC()
		o.PaidAt = &t
	}
	if ev.ExpiresAt != nil {
		o.ExpiresAt = ev.ExpiresAt
	}
	if o.LastEventAt == nil || ev.OccurredAt.After(*o.LastEventAt) {
		t := ev.OccurredAt.UTC()
		o.LastEventAt = &t
	}
	return "", true
}

// automation is what a payment status change did to the order
type automation struct {
	flag     string
	enqueued bool
}

// afterPaymentChange moves the order out of awaiting_payment when its payment ended: paid confirms
// it (cash shift, invoice, order.confirmed), expired/cancelled cancels it. Later order steps are
// never touched; money arriving for an order staff already cancelled is flagged needs_refund.
func (uc *orderUsecaseImpl) afterPaymentChange(ctx context.Context, o *shareddomain.Order, prevPayment, source, actor string) (auto automation, err error) {
	now := uc.now().UTC()
	switch {
	case o.PaymentStatus == shareddomain.PaymentPaid && prevPayment != shareddomain.PaymentPaid:
		if o.PaidAt == nil {
			o.PaidAt = &now
		}
		if o.OrderStatus != shareddomain.OrderAwaitingPayment {
			if o.OrderStatus == shareddomain.OrderCancelled {
				auto.flag = shareddomain.FlagNeedsRefund
			}
			return auto, nil
		}
		o.OrderStatus, o.ConfirmedAt = shareddomain.OrderConfirmed, &now
		if o.MethodCode == shareddomain.MethodCash {
			if err = uc.sharedUsecase.AttachCashSale(ctx, o); err != nil {
				return auto, err
			}
		}
		if _, err = uc.sharedUsecase.IssueInvoice(ctx, o); err != nil {
			return auto, err
		}
		auto.enqueued = true
		if err = uc.loadItems(ctx, o); err != nil {
			return auto, err
		}
		if err = uc.Enqueue(ctx, shareddomain.EventOrderConfirmed, o.OrderNumber,
			orderEvent(shareddomain.EventOrderConfirmed, o, now, actor, "")); err != nil {
			return auto, err
		}
		return auto, uc.LogActivity(ctx, shareddomain.OrderActivity(shareddomain.EventOrderConfirmed, o, actor,
			fmt.Sprintf("Order %s confirmed, payment of %s received", o.OrderNumber, money.FormatIDR(o.TotalAmount)), nil))

	case (o.PaymentStatus == shareddomain.PaymentExpired || o.PaymentStatus == shareddomain.PaymentCancelled) &&
		o.OrderStatus == shareddomain.OrderAwaitingPayment:
		o.OrderStatus, o.CancelledAt = shareddomain.OrderCancelled, &now
		auto.enqueued = true
		if err = uc.loadItems(ctx, o); err != nil {
			return auto, err
		}
		if err = uc.Enqueue(ctx, shareddomain.EventOrderCancelled, o.OrderNumber,
			orderEvent(shareddomain.EventOrderCancelled, o, now, actor, "payment "+o.PaymentStatus)); err != nil {
			return auto, err
		}
		return auto, uc.LogActivity(ctx, shareddomain.OrderActivity(shareddomain.EventOrderCancelled, o, actor,
			fmt.Sprintf("Order %s cancelled: payment %s", o.OrderNumber, o.PaymentStatus), map[string]any{"source": source}))
	}
	return auto, nil
}

// loadItems fills o.Items for event payloads when the order was read without them
func (uc *orderUsecaseImpl) loadItems(ctx context.Context, o *shareddomain.Order) error {
	if o.Items != nil || o.ID == 0 {
		return nil
	}
	items, err := uc.repoSQL.OrderRepo().FetchItems(ctx, o.ID)
	if items == nil {
		items = []shareddomain.OrderItem{}
	}
	o.Items = items
	return err
}

// OrderEventPayload is the payload of the order.* events
type OrderEventPayload struct {
	Event      string              `json:"event"`
	OccurredAt time.Time           `json:"occurredAt"`
	Actor      string              `json:"actor,omitempty"`
	Note       string              `json:"note,omitempty"`
	FromStatus string              `json:"fromStatus,omitempty"`
	ToStatus   string              `json:"toStatus,omitempty"`
	Order      *shareddomain.Order `json:"order"`
}

func orderEvent(event string, o *shareddomain.Order, at time.Time, actor, note string) OrderEventPayload {
	return OrderEventPayload{Event: event, OccurredAt: at.UTC(), Actor: actor, Note: note, ToStatus: o.OrderStatus, Order: o}
}
