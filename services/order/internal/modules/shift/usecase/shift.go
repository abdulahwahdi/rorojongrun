package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"monorepo/globalshared/money"
	"monorepo/globalshared/rest"
	"monorepo/services/order/internal/modules/shift/domain"
	shareddomain "monorepo/services/order/pkg/shared/domain"

	"github.com/golangid/candi/candishared"
	"github.com/golangid/candi/tracer"
	"gorm.io/gorm"
)

func (uc *shiftUsecaseImpl) OpenShift(ctx context.Context, actor string, req *domain.RequestOpenShift) (res shareddomain.CashShift, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "ShiftUsecase:OpenShift")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	cashier := strings.TrimSpace(req.CashierID)
	if cashier == "" {
		cashier = actor
	}
	if cashier == "" {
		return res, rest.NewInvalid("cashierId is required")
	}
	merchant := req.MerchantID
	if merchant == "" {
		merchant = shareddomain.DefaultMerchant
	}
	err = uc.repoSQL.WithTransaction(ctx, func(ctx context.Context) error {
		if _, err := uc.repoSQL.ShiftRepo().FindOpen(ctx, merchant, req.OutletID, cashier); err == nil {
			return rest.NewConflict("this cashier already has an open shift at this outlet")
		}
		res = shareddomain.CashShift{
			MerchantID: merchant, OutletID: req.OutletID, CashierID: cashier, Status: shareddomain.ShiftOpen,
			OpeningFloat: req.OpeningFloat, OpenedAt: uc.now().UTC(), OpenedBy: actor, Note: req.Note,
		}
		if err := uc.repoSQL.ShiftRepo().Create(ctx, &res); err != nil {
			return rest.MapDBError(err) // the partial unique index catches a concurrent open
		}
		if err := uc.sharedUsecase.Enqueue(ctx, shareddomain.EventShiftOpened, shiftKey(&res), map[string]any{
			"event": shareddomain.EventShiftOpened, "occurredAt": res.OpenedAt, "shift": res,
		}); err != nil {
			return err
		}
		return uc.sharedUsecase.LogActivity(ctx, shiftActivity(shareddomain.EventShiftOpened, &res, actor,
			fmt.Sprintf("Shift opened for cashier %s at outlet %s with a float of %s", cashier, req.OutletID, money.FormatIDR(req.OpeningFloat))))
	})
	if err == nil {
		uc.sharedUsecase.KickOutbox()
	}
	return res, err
}

func (uc *shiftUsecaseImpl) CloseShift(ctx context.Context, id int64, actor string, req *domain.RequestCloseShift) (res domain.ResponseShift, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "ShiftUsecase:CloseShift")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = uc.repoSQL.WithTransaction(ctx, func(ctx context.Context) error {
		s, err := uc.repoSQL.ShiftRepo().LockByID(ctx, id)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return rest.NewNotFound("shift not found")
		}
		if err != nil {
			return err
		}
		if s.Status != shareddomain.ShiftOpen {
			return rest.NewConflict("the shift is already closed")
		}
		totals, err := uc.repoSQL.OrderRepo().ShiftTotals(ctx, s.ID)
		if err != nil {
			return err
		}
		now := uc.now().UTC()
		applyTotals(&s, totals)
		diff := req.CountedCash - s.ExpectedCash
		s.Status, s.ClosedAt, s.ClosedBy = shareddomain.ShiftClosed, &now, actor
		s.CountedCash, s.Difference = &req.CountedCash, &diff
		if req.Note != "" {
			s.Note = strings.TrimSpace(strings.TrimSpace(s.Note) + "\n" + req.Note)
		}
		if err = uc.repoSQL.ShiftRepo().Update(ctx, &s); err != nil {
			return err
		}
		res.CashShift = s
		if err = uc.sharedUsecase.Enqueue(ctx, shareddomain.EventShiftClosed, shiftKey(&s), map[string]any{
			"event": shareddomain.EventShiftClosed, "occurredAt": now, "shift": s,
		}); err != nil {
			return err
		}
		return uc.sharedUsecase.LogActivity(ctx, shiftActivity(shareddomain.EventShiftClosed, &s, actor,
			fmt.Sprintf("Shift closed: expected %s, counted %s, difference %s",
				money.FormatIDR(s.ExpectedCash), money.FormatIDR(req.CountedCash), money.FormatIDR(diff))))
	})
	if err != nil {
		return res, err
	}
	uc.sharedUsecase.KickOutbox()
	return uc.GetShift(ctx, id)
}

func applyTotals(s *shareddomain.CashShift, t shareddomain.ShiftTotals) {
	s.CashSales, s.CashRefunds, s.OrderCount = t.CashSales, t.CashRefunds, t.OrderCount
	s.ExpectedCash = s.OpeningFloat + t.CashSales - t.CashRefunds
}

func (uc *shiftUsecaseImpl) withOrders(ctx context.Context, s shareddomain.CashShift) (res domain.ResponseShift, err error) {
	if s.Status == shareddomain.ShiftOpen { // live numbers; a closed shift keeps what was frozen at closing
		totals, err := uc.repoSQL.OrderRepo().ShiftTotals(ctx, s.ID)
		if err != nil {
			return res, err
		}
		applyTotals(&s, totals)
	}
	res.CashShift = s
	res.Orders, err = uc.repoSQL.OrderRepo().FetchAll(ctx, &shareddomain.FilterOrder{
		Filter: candishared.Filter{ShowAll: true, OrderBy: "placed_at", Sort: "asc"}, ShiftID: s.ID,
	})
	if res.Orders == nil {
		res.Orders = []shareddomain.Order{}
	}
	return res, err
}

func (uc *shiftUsecaseImpl) GetShift(ctx context.Context, id int64) (res domain.ResponseShift, err error) {
	s, err := uc.repoSQL.ShiftRepo().FindByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return res, rest.NewNotFound("shift not found")
	}
	if err != nil {
		return res, err
	}
	return uc.withOrders(ctx, s)
}

func (uc *shiftUsecaseImpl) CurrentShift(ctx context.Context, merchantID, outletID, cashierID string) (res domain.ResponseShift, err error) {
	if merchantID == "" {
		merchantID = shareddomain.DefaultMerchant
	}
	s, err := uc.repoSQL.ShiftRepo().FindOpen(ctx, merchantID, outletID, cashierID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return res, rest.NewNotFound("no open shift")
	}
	if err != nil {
		return res, err
	}
	return uc.withOrders(ctx, s)
}

func (uc *shiftUsecaseImpl) GetAllShifts(ctx context.Context, filter *domain.FilterShift) (res domain.ResponseShiftList, err error) {
	if res.Data, err = uc.repoSQL.ShiftRepo().FetchAll(ctx, filter); err != nil {
		return
	}
	if res.Data == nil {
		res.Data = []shareddomain.CashShift{}
	}
	res.Meta = candishared.NewMeta(filter.Page, filter.Limit, uc.repoSQL.ShiftRepo().Count(ctx, filter))
	return
}

func (uc *shiftUsecaseImpl) AttachCashSale(ctx context.Context, o *shareddomain.Order) error {
	if o.CashierID == "" {
		return nil // unassigned: the order list filtered on method cash without a shift shows them
	}
	s, err := uc.repoSQL.ShiftRepo().FindOpen(ctx, o.MerchantID, o.OutletID, o.CashierID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	o.ShiftID = &s.ID
	return nil
}

func (uc *shiftUsecaseImpl) AttachCashRefund(ctx context.Context, o *shareddomain.Order, cashierID string) error {
	if cashierID == "" {
		cashierID = o.CashierID
	}
	s, err := uc.repoSQL.ShiftRepo().FindOpen(ctx, o.MerchantID, o.OutletID, cashierID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	o.RefundShiftID = &s.ID
	return nil
}

func shiftKey(s *shareddomain.CashShift) string { return fmt.Sprintf("shift-%d", s.ID) }

func shiftActivity(event string, s *shareddomain.CashShift, actor, message string) shareddomain.Activity {
	return shareddomain.Activity{EventType: event, ReferenceID: shiftKey(s), ActorID: actor, Message: message,
		Metadata: map[string]any{"shiftId": s.ID, "merchantId": s.MerchantID, "outletId": s.OutletID, "cashierId": s.CashierID,
			"expectedCash": s.ExpectedCash, "countedCash": s.CountedCash, "difference": s.Difference}}
}
