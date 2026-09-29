package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"monorepo/globalshared/gormx"
	"monorepo/services/order/internal/modules/order/domain"
	shareddomain "monorepo/services/order/pkg/shared/domain"

	"github.com/golangid/candi/tracer"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type orderRepoSQL struct {
	readDB, writeDB *gorm.DB
}

// NewOrderRepoSQL repo constructor. Reads of single orders use the write connection: the event
// consumer reads right after it writes; list and report queries use the read connection.
func NewOrderRepoSQL(readDB, writeDB *gorm.DB) OrderRepository {
	return &orderRepoSQL{readDB: readDB, writeDB: writeDB}
}

func (r *orderRepoSQL) db(ctx context.Context) *gorm.DB   { return gormx.DB(ctx, r.writeDB) }
func (r *orderRepoSQL) read(ctx context.Context) *gorm.DB { return gormx.DB(ctx, r.readDB) }

func (r *orderRepoSQL) LockPayment(ctx context.Context, paymentID string) error {
	return r.db(ctx).Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "order:payment:"+paymentID).Error
}

func (r *orderRepoSQL) FindByPaymentID(ctx context.Context, paymentID string) (result shareddomain.Order, err error) {
	err = r.db(ctx).Where("payment_id = ?", paymentID).First(&result).Error
	return
}

func (r *orderRepoSQL) FindByID(ctx context.Context, id int64) (result shareddomain.Order, err error) {
	err = r.db(ctx).Where("id = ?", id).First(&result).Error
	return
}

func (r *orderRepoSQL) FindByNumber(ctx context.Context, number string) (result shareddomain.Order, err error) {
	err = r.db(ctx).Where("order_number = ?", number).First(&result).Error
	return
}

func (r *orderRepoSQL) LockByID(ctx context.Context, id int64) (result shareddomain.Order, err error) {
	err = gormx.ForUpdate(r.db(ctx)).Where("id = ?", id).First(&result).Error
	return
}

func (r *orderRepoSQL) Create(ctx context.Context, data *shareddomain.Order) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "OrderRepoSQL:Create")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	now := time.Now()
	data.CreatedAt, data.UpdatedAt = now, now
	if err = r.db(ctx).Omit("Items").Create(data).Error; err != nil {
		return err
	}
	if len(data.Items) == 0 {
		return nil
	}
	for i := range data.Items {
		data.Items[i].OrderID = data.ID
	}
	return r.db(ctx).Create(&data.Items).Error
}

func (r *orderRepoSQL) Update(ctx context.Context, data *shareddomain.Order) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "OrderRepoSQL:Update")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	data.UpdatedAt = time.Now()
	return r.db(ctx).Model(data).Select("*").Omit("id", "created_at", "Items").Updates(data).Error
}

func (r *orderRepoSQL) FetchItems(ctx context.Context, orderIDs ...int64) (result []shareddomain.OrderItem, err error) {
	if len(orderIDs) == 0 {
		return nil, nil
	}
	err = r.db(ctx).Where("order_id IN ?", orderIDs).Order("order_id, line_no").Find(&result).Error
	return
}

func (r *orderRepoSQL) EventExists(ctx context.Context, orderID int64, event, transactionID string) (bool, error) {
	var n int64
	err := r.db(ctx).Model(&shareddomain.OrderEvent{}).
		Where("order_id = ? AND event = ? AND transaction_id = ? AND source = ?", orderID, event, transactionID, shareddomain.SourceKafka).
		Count(&n).Error
	return n > 0, err
}

func (r *orderRepoSQL) InsertEvent(ctx context.Context, data *shareddomain.OrderEvent) (inserted bool, err error) {
	data.CreatedAt = time.Now()
	db := r.db(ctx)
	if data.Source == shareddomain.SourceKafka {
		// the partial unique index (order_id, event, transaction_id) WHERE source = 'kafka' dedupes redeliveries
		db = db.Clauses(clause.OnConflict{
			Columns:     []clause.Column{{Name: "order_id"}, {Name: "event"}, {Name: "transaction_id"}},
			TargetWhere: clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "source = 'kafka'"}}},
			DoNothing:   true,
		})
	}
	res := db.Create(data)
	return res.RowsAffected > 0, res.Error
}

func (r *orderRepoSQL) FetchEvents(ctx context.Context, orderID int64) (result []shareddomain.OrderEvent, err error) {
	err = r.db(ctx).Where("order_id = ?", orderID).Order("occurred_at, id").Find(&result).Error
	return
}

// filterOrders applies an order filter (list, count, summary and the keyset reads of exports)
func filterOrders(db *gorm.DB, f *shareddomain.FilterOrder) *gorm.DB {
	eq := map[string]string{
		"payment_status": f.PaymentStatus, "order_status": f.OrderStatus, "source": f.Source,
		"merchant_id": f.MerchantID, "outlet_id": f.OutletID, "cashier_id": f.CashierID,
		"channel": f.Channel, "method_code": f.MethodCode,
	}
	for col, v := range eq {
		if v != "" {
			db = db.Where(col+" = ?", v)
		}
	}
	if f.ShiftID > 0 {
		db = db.Where("(shift_id = ? OR refund_shift_id = ?)", f.ShiftID, f.ShiftID)
	}
	if f.AmountMismatch {
		db = db.Where("amount_mismatch")
	}
	if f.From != nil {
		db = db.Where("placed_at >= ?", *f.From)
	}
	if f.To != nil {
		db = db.Where("placed_at < ?", *f.To)
	}
	if s := strings.TrimSpace(f.Search); s != "" {
		like := gormx.Like(s)
		db = db.Where("(order_number ILIKE ? OR reference_id ILIKE ? OR customer_name ILIKE ? OR customer_email ILIKE ? OR customer_phone ILIKE ?)",
			like, like, like, like, like)
	}
	return db
}

func (r *orderRepoSQL) FetchAll(ctx context.Context, filter *shareddomain.FilterOrder) (result []shareddomain.Order, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "OrderRepoSQL:FetchAll")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	db := filterOrders(r.read(ctx).Model(&shareddomain.Order{}), filter)
	err = gormx.ApplyPaging(db, &filter.Filter, "placed_at", "placed_at", "paid_at", "total_amount", "order_number", "updated_at").
		Find(&result).Error
	return
}

func (r *orderRepoSQL) Count(ctx context.Context, filter *shareddomain.FilterOrder) int {
	var total int64
	filterOrders(r.read(ctx).Model(&shareddomain.Order{}), filter).Count(&total)
	return int(total)
}

func (r *orderRepoSQL) FetchAfter(ctx context.Context, filter *shareddomain.FilterOrder, afterID int64, limit int) (result []shareddomain.Order, err error) {
	err = filterOrders(r.read(ctx).Model(&shareddomain.Order{}), filter).
		Where("id > ?", afterID).Order("id").Limit(limit).Find(&result).Error
	return
}

// groupExpr maps a summary groupBy to its SQL key; day is the calendar day in the report timezone
func groupExpr(groupBy, tz string) (string, []any) {
	switch groupBy {
	case domain.GroupByDay:
		return "to_char(placed_at AT TIME ZONE ?, 'YYYY-MM-DD')", []any{tz}
	case domain.GroupByMethod:
		return "method_code", nil
	case domain.GroupByOutlet:
		return "outlet_id", nil
	case domain.GroupByCashier:
		return "cashier_id", nil
	}
	return "", nil
}

const summaryColumns = `COUNT(*) AS order_count,
	COUNT(*) FILTER (WHERE payment_status = 'paid') AS paid_count,
	COALESCE(SUM(subtotal) FILTER (WHERE payment_status = 'paid'), 0) AS subtotal,
	COALESCE(SUM(tax_amount) FILTER (WHERE payment_status = 'paid'), 0) AS tax_amount,
	COALESCE(SUM(rounding_adjustment) FILTER (WHERE payment_status = 'paid'), 0) AS rounding_adjustment,
	COALESCE(SUM(amount) FILTER (WHERE payment_status = 'paid'), 0) AS gross,
	COALESCE(SUM(fee) FILTER (WHERE payment_status = 'paid'), 0) AS fee,
	COALESCE(SUM(total_amount) FILTER (WHERE payment_status = 'paid'), 0) AS total_amount,
	COUNT(*) FILTER (WHERE order_status = 'refunded' OR (order_status = 'cancelled' AND payment_status = 'paid')) AS refunded_count,
	COALESCE(SUM(total_amount) FILTER (WHERE order_status = 'refunded' OR (order_status = 'cancelled' AND payment_status = 'paid')), 0) AS refunded_amount`

func (r *orderRepoSQL) Summary(ctx context.Context, f *domain.FilterSummary) (totals domain.SummaryTotals, groups []domain.SummaryGroup, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "OrderRepoSQL:Summary")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	base := func() *gorm.DB { return filterOrders(r.read(ctx).Model(&shareddomain.Order{}), &f.FilterOrder) }
	if err = base().Select(summaryColumns).Scan(&totals).Error; err != nil {
		return
	}

	var byPayment, byOrder []domain.StatusCount
	if err = base().Select("payment_status AS status, COUNT(*) AS count").Group("payment_status").Order("payment_status").Scan(&byPayment).Error; err != nil {
		return
	}
	if err = base().Select("order_status AS status, COUNT(*) AS count").Group("order_status").Order("order_status").Scan(&byOrder).Error; err != nil {
		return
	}
	totals.ByPaymentStatus, totals.ByOrderStatus = byPayment, byOrder

	expr, args := groupExpr(f.GroupBy, f.Timezone)
	if expr == "" {
		return
	}
	err = base().Select(fmt.Sprintf("%s AS key, %s", expr, summaryColumns), args...).
		Group("key").Order("key").Scan(&groups).Error
	return
}

func (r *orderRepoSQL) ShiftTotals(ctx context.Context, shiftID int64) (totals shareddomain.ShiftTotals, err error) {
	err = r.db(ctx).Model(&shareddomain.Order{}).Select(`
		COALESCE(SUM(total_amount) FILTER (WHERE shift_id = @id AND payment_status = 'paid'), 0) AS cash_sales,
		COALESCE(SUM(total_amount) FILTER (WHERE refund_shift_id = @id), 0) AS cash_refunds,
		COUNT(*) FILTER (WHERE shift_id = @id AND payment_status = 'paid') AS order_count`,
		map[string]any{"id": shiftID}).
		Where("(shift_id = ? OR refund_shift_id = ?)", shiftID, shiftID).
		Scan(&totals).Error
	return
}
