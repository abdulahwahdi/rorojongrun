package usecase

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"monorepo/globalshared/rest"
	"monorepo/services/order/internal/modules/order/domain"
	shareddomain "monorepo/services/order/pkg/shared/domain"

	"github.com/golangid/candi/tracer"
	"gorm.io/gorm"
)

// stepRank orders the forward steps after payment; staff may skip steps forward, never go back
var stepRank = map[string]int{
	shareddomain.OrderConfirmed: 1, shareddomain.OrderPreparing: 2, shareddomain.OrderReady: 3, shareddomain.OrderCompleted: 4,
}

// AllowedOrderStatuses are the order statuses staff can move an order to from its current state
func AllowedOrderStatuses(o *shareddomain.Order) []string {
	switch o.OrderStatus {
	case shareddomain.OrderAwaitingPayment:
		// only a walk-out: leaving awaiting_payment otherwise needs the payment to be paid
		return []string{shareddomain.OrderCancelled}
	case shareddomain.OrderConfirmed, shareddomain.OrderPreparing, shareddomain.OrderReady:
		var next []string
		for _, s := range []string{shareddomain.OrderPreparing, shareddomain.OrderReady, shareddomain.OrderCompleted} {
			if stepRank[s] > stepRank[o.OrderStatus] {
				next = append(next, s)
			}
		}
		return append(next, shareddomain.OrderCancelled)
	case shareddomain.OrderCompleted:
		if o.PaymentStatus == shareddomain.PaymentPaid {
			return []string{shareddomain.OrderRefunded}
		}
	}
	return []string{}
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// lockOrder resolves an id or order number and locks the order row; call inside WithTransaction
func (uc *orderUsecaseImpl) lockOrder(ctx context.Context, idOrNumber string) (shareddomain.Order, error) {
	repo := uc.repoSQL.OrderRepo()
	id, perr := strconv.ParseInt(idOrNumber, 10, 64)
	if perr != nil {
		o, err := repo.FindByNumber(ctx, idOrNumber)
		if err != nil {
			return o, notFound(err)
		}
		id = o.ID
	}
	o, err := repo.LockByID(ctx, id)
	return o, notFound(err)
}

func notFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return rest.NewNotFound("order not found")
	}
	return err
}

func (uc *orderUsecaseImpl) UpdateStatus(ctx context.Context, idOrNumber, actor string, req *domain.RequestUpdateStatus) (res domain.ResponseOrder, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "OrderUsecase:UpdateStatus")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	req.Note = strings.TrimSpace(req.Note)
	var orderID int64
	err = uc.repoSQL.WithTransaction(ctx, func(ctx context.Context) error {
		o, err := uc.lockOrder(ctx, idOrNumber)
		if err != nil {
			return err
		}
		orderID = o.ID
		if req.Version != o.Version {
			return rest.NewConflict(fmt.Sprintf("the order changed (version %d, you sent %d), reload it", o.Version, req.Version))
		}
		if !contains(AllowedOrderStatuses(&o), req.Status) {
			return rest.NewConflict(fmt.Sprintf("order status cannot go from %s to %s", o.OrderStatus, req.Status))
		}
		paid := o.PaymentStatus == shareddomain.PaymentPaid
		if (req.Status == shareddomain.OrderRefunded || (req.Status == shareddomain.OrderCancelled && paid)) && req.Note == "" {
			return rest.NewInvalid("a note is required to refund or cancel a paid order")
		}

		from, now := o.OrderStatus, uc.now().UTC()
		o.OrderStatus = req.Status
		specific := ""
		switch req.Status {
		case shareddomain.OrderCompleted:
			o.CompletedAt, specific = &now, shareddomain.EventOrderCompleted
		case shareddomain.OrderCancelled:
			o.CancelledAt, specific = &now, shareddomain.EventOrderCancelled
		case shareddomain.OrderRefunded:
			o.RefundedAt, specific = &now, shareddomain.EventOrderRefunded
		}
		// money goes back: reverse the invoice, take cash refunds out of the refunding cashier's shift
		if paid && (req.Status == shareddomain.OrderCancelled || req.Status == shareddomain.OrderRefunded) {
			if _, _, err = uc.sharedUsecase.IssueCreditNote(ctx, &o); err != nil {
				return err
			}
			if o.MethodCode == shareddomain.MethodCash {
				if err = uc.sharedUsecase.AttachCashRefund(ctx, &o, actor); err != nil {
					return err
				}
			}
		}
		o.Version++
		if err = uc.repoSQL.OrderRepo().Update(ctx, &o); err != nil {
			return err
		}
		if _, err = uc.repoSQL.OrderRepo().InsertEvent(ctx, &shareddomain.OrderEvent{
			OrderID: o.ID, Event: shareddomain.EventOrderStatusUpdated, Source: shareddomain.SourceManual,
			FromStatus: from, ToStatus: o.OrderStatus, Amount: o.Amount, Fee: o.Fee, TotalAmount: o.TotalAmount,
			MethodCode: o.MethodCode, Actor: actor, Note: req.Note, OccurredAt: now,
		}); err != nil {
			return err
		}

		if err = uc.loadItems(ctx, &o); err != nil {
			return err
		}
		payload := orderEvent(shareddomain.EventOrderStatusUpdated, &o, now, actor, req.Note)
		payload.FromStatus = from
		if err = uc.Enqueue(ctx, shareddomain.EventOrderStatusUpdated, o.OrderNumber, payload); err != nil {
			return err
		}
		if specific != "" {
			payload.Event = specific
			if err = uc.Enqueue(ctx, specific, o.OrderNumber, payload); err != nil {
				return err
			}
		}
		msg := fmt.Sprintf("Order %s moved from %s to %s", o.OrderNumber, from, o.OrderStatus)
		if req.Note != "" {
			msg += ": " + req.Note
		}
		return uc.LogActivity(ctx, shareddomain.OrderActivity(shareddomain.EventOrderStatusUpdated, &o, actor, msg,
			map[string]any{"fromStatus": from, "toStatus": o.OrderStatus}))
	})
	if err != nil {
		return res, err
	}
	uc.afterCommit()
	return uc.GetOrder(ctx, strconv.FormatInt(orderID, 10))
}

var overridableStatuses = map[string]bool{
	shareddomain.PaymentPending: true, shareddomain.PaymentPaid: true,
	shareddomain.PaymentExpired: true, shareddomain.PaymentCancelled: true,
}

func (uc *orderUsecaseImpl) OverridePaymentStatus(ctx context.Context, idOrNumber, actor string, req *domain.RequestOverridePaymentStatus) (res domain.ResponseOrder, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "OrderUsecase:OverridePaymentStatus")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	req.Note = strings.TrimSpace(req.Note)
	if req.Note == "" {
		return res, rest.NewInvalid("a note is required to override a payment status")
	}
	if !overridableStatuses[req.Status] {
		return res, rest.NewInvalid("payment status can be set to pending, paid, expired or cancelled")
	}

	var orderID int64
	err = uc.repoSQL.WithTransaction(ctx, func(ctx context.Context) error {
		o, err := uc.lockOrder(ctx, idOrNumber)
		if err != nil {
			return err
		}
		orderID = o.ID
		if req.Version != o.Version {
			return rest.NewConflict(fmt.Sprintf("the order changed (version %d, you sent %d), reload it", o.Version, req.Version))
		}
		if o.PaymentStatus == req.Status {
			return rest.NewConflict("the payment status is already " + req.Status)
		}
		if o.PaymentStatus == shareddomain.PaymentPaid {
			return rest.NewConflict("a paid payment cannot be changed by hand, refund or cancel the order instead")
		}

		from, now := o.PaymentStatus, uc.now().UTC()
		o.PaymentStatus, o.PaymentStatusSource = req.Status, shareddomain.SourceManual
		if req.Status == shareddomain.PaymentPaid {
			o.PaidAt = &now
			if req.MethodCode != "" {
				o.MethodCode = req.MethodCode
			}
		}
		auto, err := uc.afterPaymentChange(ctx, &o, from, shareddomain.SourceManual, actor)
		if err != nil {
			return err
		}
		o.Version++
		if err = uc.repoSQL.OrderRepo().Update(ctx, &o); err != nil {
			return err
		}
		if _, err = uc.repoSQL.OrderRepo().InsertEvent(ctx, &shareddomain.OrderEvent{
			OrderID: o.ID, Event: shareddomain.EventPaymentStatusOverridden, Source: shareddomain.SourceManual,
			FromStatus: from, ToStatus: o.PaymentStatus, Amount: o.Amount, Fee: o.Fee, TotalAmount: o.TotalAmount,
			MethodCode: o.MethodCode, Actor: actor, Note: req.Note, Flag: auto.flag, OccurredAt: now,
		}); err != nil {
			return err
		}
		return uc.LogActivity(ctx, shareddomain.OrderActivity(shareddomain.EventPaymentStatusOverridden, &o, actor,
			fmt.Sprintf("Payment status of order %s set by hand from %s to %s: %s", o.OrderNumber, from, o.PaymentStatus, req.Note),
			map[string]any{"fromStatus": from, "toStatus": o.PaymentStatus}))
	})
	if err != nil {
		return res, err
	}
	uc.afterCommit()
	return uc.GetOrder(ctx, strconv.FormatInt(orderID, 10))
}
