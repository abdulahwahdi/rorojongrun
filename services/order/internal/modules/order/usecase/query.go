package usecase

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"monorepo/globalshared/rest"
	"monorepo/services/order/internal/modules/order/domain"
	shareddomain "monorepo/services/order/pkg/shared/domain"

	"github.com/golangid/candi/candishared"
	"github.com/golangid/candi/tracer"
)

func (uc *orderUsecaseImpl) GetAllOrders(ctx context.Context, filter *shareddomain.FilterOrder) (res domain.ResponseOrderList, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "OrderUsecase:GetAllOrders")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	if _, err = uc.ResolveDates(ctx, filter.MerchantID, &filter.DateRange); err != nil {
		return
	}
	repo := uc.repoSQL.OrderRepo()
	if res.Data, err = repo.FetchAll(ctx, filter); err != nil {
		return
	}
	if res.Data == nil {
		res.Data = []shareddomain.Order{}
	}
	res.Meta = candishared.NewMeta(filter.Page, filter.Limit, repo.Count(ctx, filter))
	return
}

func (uc *orderUsecaseImpl) GetOrder(ctx context.Context, idOrNumber string) (res domain.ResponseOrder, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "OrderUsecase:GetOrder")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	repo := uc.repoSQL.OrderRepo()
	var o shareddomain.Order
	if id, perr := strconv.ParseInt(idOrNumber, 10, 64); perr == nil {
		o, err = repo.FindByID(ctx, id)
	} else {
		o, err = repo.FindByNumber(ctx, idOrNumber)
	}
	if err != nil {
		return res, notFound(err)
	}
	if err = uc.loadItems(ctx, &o); err != nil {
		return
	}
	res.Order = o
	if res.Events, err = repo.FetchEvents(ctx, o.ID); err != nil {
		return
	}
	if res.Invoices, err = uc.sharedUsecase.InvoicesOfOrder(ctx, o.ID); err != nil {
		return
	}
	if res.Events == nil {
		res.Events = []shareddomain.OrderEvent{}
	}
	if res.Invoices == nil {
		res.Invoices = []shareddomain.Invoice{}
	}
	res.AllowedStatuses = AllowedOrderStatuses(&o)
	return
}

func (uc *orderUsecaseImpl) GetSummary(ctx context.Context, filter *domain.FilterSummary) (res domain.ResponseSummary, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "OrderUsecase:GetSummary")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	if filter.Timezone, err = uc.ResolveDates(ctx, filter.MerchantID, &filter.DateRange); err != nil {
		return
	}
	totals, groups, err := uc.repoSQL.OrderRepo().Summary(ctx, filter)
	if err != nil {
		return
	}
	totals.Net = totals.TotalAmount - totals.RefundedAmount
	for i := range groups {
		groups[i].Net = groups[i].TotalAmount - groups[i].RefundedAmount
	}
	if groups == nil {
		groups = []domain.SummaryGroup{}
	}
	if totals.ByPaymentStatus == nil {
		totals.ByPaymentStatus = []domain.StatusCount{}
	}
	if totals.ByOrderStatus == nil {
		totals.ByOrderStatus = []domain.StatusCount{}
	}
	res = domain.ResponseSummary{Timezone: filter.Timezone, GroupBy: filter.GroupBy, Totals: totals, Groups: groups}
	if filter.From != nil {
		res.From = filter.From.Format(time.RFC3339)
	}
	if filter.To != nil {
		res.To = filter.To.Format(time.RFC3339)
	}
	return
}

// ResolveDates parses a date range in the merchant's timezone (the default merchant's when none is
// given). A date-only DateTo includes that whole day.
func (uc *orderUsecaseImpl) ResolveDates(ctx context.Context, merchantID string, r *shareddomain.DateRange) (tz string, err error) {
	m, err := uc.sharedUsecase.MerchantSettings(ctx, merchantID)
	if err != nil {
		return "", err
	}
	loc := m.Location()
	parse := func(s string, end bool) (*time.Time, error) {
		s = strings.TrimSpace(s)
		if s == "" {
			return nil, nil
		}
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return &t, nil
		}
		d, err := time.ParseInLocation("2006-01-02", s, loc)
		if err != nil {
			return nil, errors.New("dates are YYYY-MM-DD or RFC3339: " + s)
		}
		if end {
			d = d.AddDate(0, 0, 1)
		}
		return &d, nil
	}
	if r.From, err = parse(r.DateFrom, false); err != nil {
		return "", rest.NewInvalid(err.Error())
	}
	if r.To, err = parse(r.DateTo, true); err != nil {
		return "", rest.NewInvalid(err.Error())
	}
	return loc.String(), nil
}
