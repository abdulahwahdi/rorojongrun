// Package test runs the order service's usecases against a real Postgres (the SQL, the locks and
// the unique indexes are part of what is tested). It is skipped unless ORDER_TEST_DSN points to a
// throwaway database, which is wiped:
//
//	ORDER_TEST_DSN='postgres://user:pass@localhost:55432/order_test?sslmode=disable' go test ./services/order/test/...
package test

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"monorepo/globalshared/rest"
	exportdomain "monorepo/services/order/internal/modules/export/domain"
	ucexport "monorepo/services/order/internal/modules/export/usecase"
	merchantdomain "monorepo/services/order/internal/modules/merchant/domain"
	orderdomain "monorepo/services/order/internal/modules/order/domain"
	shiftdomain "monorepo/services/order/internal/modules/shift/domain"
	"monorepo/services/order/pkg/shared"
	shareddomain "monorepo/services/order/pkg/shared/domain"
	"monorepo/services/order/pkg/shared/repository"
	"monorepo/services/order/pkg/shared/usecase"

	"github.com/golangid/candi/candishared"
	"github.com/golangid/candi/codebase/factory/dependency"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testDB struct{ db *sql.DB }

func (t testDB) ReadDB() *sql.DB                      { return t.db }
func (t testDB) WriteDB() *sql.DB                     { return t.db }
func (t testDB) Health() map[string]error             { return nil }
func (t testDB) Disconnect(ctx context.Context) error { return t.db.Close() }

var (
	db  *sql.DB
	uc  usecase.Usecase
	ctx = context.Background()
)

func TestMain(m *testing.M) {
	dsn := os.Getenv("ORDER_TEST_DSN")
	if dsn == "" {
		fmt.Println("ORDER_TEST_DSN not set, skipping the order integration tests")
		os.Exit(0)
	}
	var err error
	if db, err = sql.Open("postgres", dsn); err != nil {
		panic(err)
	}
	if err = migrate(db); err != nil {
		panic(err)
	}
	dir, _ := os.MkdirTemp("", "order-exports")
	defer os.RemoveAll(dir)
	shared.SetEnv(shared.Environment{ExportStorageDir: dir, ExportMaxRows: 50})

	deps := dependency.InitDependency(dependency.SetSQLDatabase(testDB{db}))
	repository.SetSharedRepository(deps)
	usecase.SetSharedUsecase(deps)
	uc = usecase.GetSharedUsecase()
	os.Exit(m.Run())
}

// migrate wipes the database and applies the Up part of every migration
func migrate(db *sql.DB) error {
	if _, err := db.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public"); err != nil {
		return err
	}
	files, _ := filepath.Glob("../cmd/migration/migrations/*.sql")
	sort.Strings(files)
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		up := strings.SplitN(strings.SplitN(string(b), "-- +goose Down", 2)[0], "-- +goose Up", 2)[1]
		if _, err = db.Exec(up); err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
	}
	return nil
}

func status(err error) int {
	if err == nil {
		return 0
	}
	return rest.HTTPStatus(err)
}

// ---- builders

type pay struct {
	id, ref, merchant, cashier, method string
	amount                             int64
	items                              []map[string]any
	email                              string
	created                            time.Time
}

func newPay(merchant string, amount int64) *pay {
	id := uuid.NewString()
	return &pay{id: id, ref: "REF-" + id[:8], merchant: merchant, cashier: "cashier-1", method: "qris", amount: amount,
		items: []map[string]any{{"name": "Kopi", "price": amount, "quantity": 1}}, email: "budi@example.com",
		created: time.Now().UTC().Add(-time.Minute)}
}

func (p *pay) event(event, status, txn string, at time.Time) *orderdomain.PaymentEvent {
	raw, _ := json.Marshal(map[string]any{
		"event": event, "paymentId": p.id, "transactionId": txn, "source": "pos", "referenceId": p.ref,
		"description": "Table 4", "status": status, "amount": p.amount, "fee": 0, "totalAmount": p.amount,
		"currency": "IDR", "methodCode": p.method,
		"metadata": map[string]any{"merchantId": p.merchant, "outletId": "O1", "cashierId": p.cashier, "channel": "pos"},
		"customer": map[string]any{"name": "Budi", "email": p.email}, "items": p.items,
		"createdAt": p.created, "occurredAt": at,
	})
	var ev orderdomain.PaymentEvent
	_ = json.Unmarshal(raw, &ev)
	if status == "paid" {
		t := at
		ev.PaidAt = &t
	}
	return &ev
}

func book(t *testing.T, ev *orderdomain.PaymentEvent) {
	t.Helper()
	require.NoError(t, uc.Order().RecordPaymentEvent(ctx, ev, orderdomain.KafkaSource{Topic: ev.Event}))
}

func orderOf(t *testing.T, p *pay) orderdomain.ResponseOrder {
	t.Helper()
	var id int64
	require.NoError(t, db.QueryRow("SELECT id FROM orders WHERE payment_id = $1", p.id).Scan(&id))
	res, err := uc.Order().GetOrder(ctx, fmt.Sprint(id))
	require.NoError(t, err)
	return res
}

func outboxCount(t *testing.T, eventType, key string) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM order_outbox WHERE event_type = $1 AND key = $2", eventType, key).Scan(&n))
	return n
}

func activityCount(t *testing.T, eventType, ref string) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM order_outbox WHERE event_type = 'activity_log' AND topic = 'activity.requested'
		AND key = $1 AND payload->>'eventType' = $2 AND payload->>'serviceName' = 'order'`, ref, eventType).Scan(&n))
	return n
}

func eventsOf(o orderdomain.ResponseOrder) (list []string) {
	for _, e := range o.Events {
		s := e.Event
		if e.Flag != "" {
			s += "!" + e.Flag
		}
		list = append(list, s)
	}
	return
}

func merchant(t *testing.T, id string) {
	t.Helper()
	_, err := uc.Merchant().SaveMerchant(ctx, id, &merchantdomain.RequestSaveMerchant{
		Name: "Kopi Kita", Address: "Jl. Sudirman 1", TaxID: "01.234.567.8-901.000", OrderPrefix: "KOP",
		TaxRate: 11, TaxMode: "exclusive", RoundingMode: "nearest", RoundingUnit: 100,
	})
	require.NoError(t, err)
}

// uniq gives each test run its own merchant, so runs (go test -count=N) do not see each other's data
func uniq(prefix string) string { return prefix + "-" + uuid.NewString()[:6] }

// ---- tests

func Test_Ledger_OutOfOrderAndDuplicates(t *testing.T) {
	m1 := uniq("M1")
	merchant(t, m1)
	p := newPay(m1, 53800)
	p.items = []map[string]any{{"name": "Kopi", "price": 18000, "quantity": 2}, {"name": "Roti", "price": 12500, "quantity": 1}}
	t0 := time.Now().UTC()

	// payment.completed arrives before payment.created: the order is still complete
	book(t, p.event("payment.completed", "paid", "txn-1", t0))
	o := orderOf(t, p)
	assert.Regexp(t, regexp.MustCompile(`^KOP-\d{8}-\d{6}$`), o.OrderNumber)
	assert.Equal(t, "paid", o.PaymentStatus)
	assert.Equal(t, "confirmed", o.OrderStatus)
	assert.Len(t, o.Items, 2)
	assert.EqualValues(t, 48500, o.Subtotal)
	assert.EqualValues(t, 5335, o.TaxAmount)
	assert.EqualValues(t, -35, o.RoundingAdjustment)
	assert.EqualValues(t, 53800, o.ExpectedTotal)
	assert.False(t, o.AmountMismatch)
	require.Len(t, o.Invoices, 1)
	assert.Regexp(t, regexp.MustCompile(`^INV/`+m1+`/\d{6}/000001$`), o.Invoices[0].Number)
	assert.Equal(t, "Kopi Kita", o.Invoices[0].SellerName)

	// the older payment.created is recorded but does not move anything back
	book(t, p.event("payment.created", "pending", "", t0.Add(-time.Minute)))
	// a redelivery is booked once
	book(t, p.event("payment.completed", "paid", "txn-1", t0))
	o = orderOf(t, p)
	assert.Equal(t, "paid", o.PaymentStatus)
	assert.Equal(t, []string{"payment.created!stale", "payment.completed"}, eventsOf(o))
	assert.Equal(t, 1, o.Version)

	for _, e := range []string{"order.created", "order.confirmed"} {
		assert.Equal(t, 1, outboxCount(t, e, o.OrderNumber), e)
		assert.Equal(t, 1, activityCount(t, e, o.OrderNumber), "activity "+e)
	}
	assert.Equal(t, 1, outboxCount(t, "order.invoice_issued", o.Invoices[0].Number))
	assert.Equal(t, 1, outboxCount(t, "notification_email", o.Invoices[0].Number), "the invoice is emailed")
	assert.Equal(t, 2, activityCount(t, "payment.completed", o.OrderNumber)+activityCount(t, "payment.created", o.OrderNumber),
		"every booked payment event is in the audit trail, redeliveries are not")

	var raw []byte
	var payload map[string]any
	require.NoError(t, db.QueryRow("SELECT payload FROM order_outbox WHERE event_type = 'order.confirmed' AND key = $1", o.OrderNumber).Scan(&raw))
	require.NoError(t, json.Unmarshal(raw, &payload))
	assert.Len(t, payload["order"].(map[string]any)["items"], 2, "order events carry the items")
}

func Test_Ledger_NumbersHaveNoGaps(t *testing.T) {
	m2 := uniq("M2")
	merchant(t, m2)
	var numbers []string
	for i := 0; i < 3; i++ {
		p := newPay(m2, 10000)
		book(t, p.event("payment.created", "pending", "", time.Now().UTC()))
		numbers = append(numbers, orderOf(t, p).OrderNumber)
	}
	for i := 1; i < len(numbers); i++ { // consecutive: the KOP sequence is shared with other merchants using KOP
		var prev, cur int
		fmt.Sscanf(numbers[i-1][len(numbers[i-1])-6:], "%d", &prev)
		fmt.Sscanf(numbers[i][len(numbers[i])-6:], "%d", &cur)
		assert.Equal(t, prev+1, cur, numbers)
	}
	// a merchant without settings uses the default settings, numbered on its own
	p := newPay(uniq("NEW"), 10000)
	book(t, p.event("payment.created", "pending", "", time.Now().UTC()))
	assert.True(t, strings.HasPrefix(orderOf(t, p).OrderNumber, "ORD-"))
	assert.True(t, orderOf(t, p).AmountMismatch == false)
}

func Test_Ledger_ExpiryAndCheckoutAttempts(t *testing.T) {
	p := newPay("", 25000)
	t0 := time.Now().UTC()
	book(t, p.event("payment.created", "pending", "", t0))
	book(t, p.event("payment.checkout_started", "processing", "a1", t0.Add(time.Second)))
	book(t, p.event("payment.checkout_started", "processing", "a2", t0.Add(2*time.Second)))
	o := orderOf(t, p)
	assert.Equal(t, 2, o.AttemptCount)
	assert.Equal(t, "a2", o.TransactionID)
	assert.Equal(t, "awaiting_payment", o.OrderStatus)

	book(t, p.event("payment.expired", "expired", "", t0.Add(time.Hour)))
	o = orderOf(t, p)
	assert.Equal(t, "expired", o.PaymentStatus)
	assert.Equal(t, "cancelled", o.OrderStatus)
	assert.Empty(t, o.Invoices)
	assert.Equal(t, 1, outboxCount(t, "order.cancelled", o.OrderNumber))
}

func Test_Lifecycle_ManualSteps_Refund_And_CashShift(t *testing.T) {
	m3 := uniq("M3")
	merchant(t, m3)
	shift, err := uc.Shift().OpenShift(ctx, "cashier-3", &shiftdomain.RequestOpenShift{MerchantID: m3, OutletID: "O1", OpeningFloat: 200000})
	require.NoError(t, err)
	_, err = uc.Shift().OpenShift(ctx, "cashier-3", &shiftdomain.RequestOpenShift{MerchantID: m3, OutletID: "O1"})
	assert.Equal(t, http.StatusConflict, status(err), "one open shift per cashier and outlet")

	p := newPay(m3, 55500)
	p.cashier, p.method = "cashier-3", "cash"
	p.items = []map[string]any{{"name": "Nasi", "price": 50000, "quantity": 1}}
	book(t, p.event("payment.completed", "paid", "c1", time.Now().UTC()))
	o := orderOf(t, p)
	require.NotNil(t, o.ShiftID, "a cash sale lands in the cashier's open shift")
	assert.Equal(t, shift.ID, *o.ShiftID)

	id := fmt.Sprint(o.ID)
	_, err = uc.Order().UpdateStatus(ctx, id, "staff-1", &orderdomain.RequestUpdateStatus{Status: "ready", Version: o.Version + 1})
	assert.Equal(t, http.StatusConflict, status(err), "stale version")
	res, err := uc.Order().UpdateStatus(ctx, id, "staff-1", &orderdomain.RequestUpdateStatus{Status: "preparing", Version: o.Version})
	require.NoError(t, err)
	_, err = uc.Order().UpdateStatus(ctx, id, "staff-1", &orderdomain.RequestUpdateStatus{Status: "confirmed", Version: res.Version})
	assert.Equal(t, http.StatusConflict, status(err), "never backwards")
	res, err = uc.Order().UpdateStatus(ctx, id, "staff-1", &orderdomain.RequestUpdateStatus{Status: "completed", Version: res.Version})
	require.NoError(t, err)
	assert.NotNil(t, res.CompletedAt)
	_, err = uc.Order().UpdateStatus(ctx, id, "staff-1", &orderdomain.RequestUpdateStatus{Status: "refunded", Version: res.Version})
	assert.Equal(t, http.StatusBadRequest, status(err), "a refund needs a note")
	res, err = uc.Order().UpdateStatus(ctx, res.OrderNumber, "cashier-3", &orderdomain.RequestUpdateStatus{Status: "refunded", Note: "cold food", Version: res.Version})
	require.NoError(t, err)
	assert.Equal(t, "refunded", res.OrderStatus)
	require.Len(t, res.Invoices, 2)
	assert.Equal(t, "credited", res.Invoices[0].Status)
	assert.Equal(t, "credit_note", res.Invoices[1].Type)
	assert.Regexp(t, regexp.MustCompile(`^CN/`+m3+`/\d{6}/000001$`), res.Invoices[1].Number)
	assert.Equal(t, res.Invoices[0].ID, *res.Invoices[1].RefInvoiceID)
	require.NotNil(t, res.RefundShiftID)
	assert.Equal(t, []string{"payment.completed", "order.status_updated", "order.status_updated", "order.status_updated"}, eventsOf(res))
	assert.Equal(t, 3, activityCount(t, "order.status_updated", res.OrderNumber))
	assert.Equal(t, 1, outboxCount(t, "order.refunded", res.OrderNumber))

	// a second cash sale that stays
	p2 := newPay(m3, 20000)
	p2.cashier, p2.method = "cashier-3", "cash"
	book(t, p2.event("payment.completed", "paid", "c2", time.Now().UTC()))

	live, err := uc.Shift().CurrentShift(ctx, m3, "O1", "cashier-3")
	require.NoError(t, err)
	assert.EqualValues(t, 55500+20000, live.CashSales)
	assert.EqualValues(t, 55500, live.CashRefunds)
	assert.EqualValues(t, 200000+20000, live.ExpectedCash)
	assert.Len(t, live.Orders, 2)

	closed, err := uc.Shift().CloseShift(ctx, shift.ID, "cashier-3", &shiftdomain.RequestCloseShift{CountedCash: 219000})
	require.NoError(t, err)
	assert.Equal(t, "closed", closed.Status)
	assert.EqualValues(t, -1000, *closed.Difference)
	_, err = uc.Shift().CloseShift(ctx, shift.ID, "cashier-3", &shiftdomain.RequestCloseShift{CountedCash: 1})
	assert.Equal(t, http.StatusConflict, status(err))
	assert.Equal(t, 1, activityCount(t, "order.shift_closed", fmt.Sprintf("shift-%d", shift.ID)))
}

func Test_Lifecycle_WalkOutAndOverrides(t *testing.T) {
	t0 := time.Now().UTC()

	// staff cancel an unpaid order, then the money arrives anyway
	p := newPay("", 15000)
	book(t, p.event("payment.created", "pending", "", t0))
	o := orderOf(t, p)
	assert.Equal(t, []string{"cancelled"}, o.AllowedStatuses)
	_, err := uc.Order().UpdateStatus(ctx, o.OrderNumber, "staff", &orderdomain.RequestUpdateStatus{Status: "cancelled", Version: o.Version})
	require.NoError(t, err)
	book(t, p.event("payment.completed", "paid", "t1", t0.Add(time.Minute)))
	o = orderOf(t, p)
	assert.Equal(t, "paid", o.PaymentStatus)
	assert.Equal(t, "cancelled", o.OrderStatus, "payment never reopens an order staff cancelled")
	assert.Contains(t, eventsOf(o), "payment.completed!needs_refund")

	// an admin marks a payment cancelled by hand; a late paid event does not override it
	p2 := newPay("", 15000)
	book(t, p2.event("payment.created", "pending", "", t0))
	o2 := orderOf(t, p2)
	_, err = uc.Order().OverridePaymentStatus(ctx, o2.OrderNumber, "admin", &orderdomain.RequestOverridePaymentStatus{Status: "cancelled", Version: o2.Version})
	assert.Equal(t, http.StatusBadRequest, status(err), "a note is required")
	res, err := uc.Order().OverridePaymentStatus(ctx, o2.OrderNumber, "admin", &orderdomain.RequestOverridePaymentStatus{Status: "cancelled", Note: "customer left", Version: o2.Version})
	require.NoError(t, err)
	assert.Equal(t, "manual", res.PaymentStatusSource)
	assert.Equal(t, "cancelled", res.OrderStatus)
	book(t, p2.event("payment.completed", "paid", "t2", t0.Add(time.Minute)))
	o2 = orderOf(t, p2)
	assert.Equal(t, "cancelled", o2.PaymentStatus)
	assert.Contains(t, eventsOf(o2), "payment.completed!ignored_manual")

	// a lost callback: the admin marks it paid, the order is confirmed and invoiced like from Kafka
	p3 := newPay("", 30000)
	book(t, p3.event("payment.created", "pending", "", t0))
	o3 := orderOf(t, p3)
	res, err = uc.Order().OverridePaymentStatus(ctx, o3.OrderNumber, "admin", &orderdomain.RequestOverridePaymentStatus{Status: "paid", Note: "paid by transfer, callback lost", MethodCode: "bank_transfer", Version: o3.Version})
	require.NoError(t, err)
	assert.Equal(t, "confirmed", res.OrderStatus)
	assert.Equal(t, "bank_transfer", res.MethodCode)
	assert.Len(t, res.Invoices, 1)
	_, err = uc.Order().OverridePaymentStatus(ctx, o3.OrderNumber, "admin", &orderdomain.RequestOverridePaymentStatus{Status: "pending", Note: "x", Version: res.Version})
	assert.Equal(t, http.StatusConflict, status(err), "a paid payment is not changed by hand")
	assert.Equal(t, 1, activityCount(t, "order.payment_status_overridden", o3.OrderNumber))
}

func Test_Summary(t *testing.T) {
	m4 := uniq("M4")
	merchant(t, m4)
	for i, st := range []string{"paid", "paid", "expired"} {
		p := newPay(m4, int64(10000*(i+1)))
		ev := map[string]string{"paid": "payment.completed", "expired": "payment.expired"}[st]
		book(t, p.event(ev, st, fmt.Sprint("s", i), time.Now().UTC()))
	}
	res, err := uc.Order().GetSummary(ctx, &orderdomain.FilterSummary{
		FilterOrder: shareddomain.FilterOrder{MerchantID: m4, DateRange: shareddomain.DateRange{DateFrom: time.Now().Format("2006-01-02"), DateTo: time.Now().Format("2006-01-02")}},
		GroupBy:     "day",
	})
	require.NoError(t, err)
	assert.EqualValues(t, 3, res.Totals.OrderCount)
	assert.EqualValues(t, 2, res.Totals.PaidCount)
	assert.EqualValues(t, 30000, res.Totals.TotalAmount)
	assert.EqualValues(t, 30000, res.Totals.Net)
	assert.Equal(t, "Asia/Jakarta", res.Timezone)
	require.Len(t, res.Groups, 1)
	assert.Equal(t, time.Now().In(time.FixedZone("WIB", 7*3600)).Format("2006-01-02"), res.Groups[0].Key)
}

func Test_Export(t *testing.T) {
	m5 := uniq("M5")
	merchant(t, m5)
	for i := 0; i < 3; i++ {
		p := newPay(m5, 10000)
		p.items = []map[string]any{{"name": "A", "price": 5000, "quantity": 1}, {"name": "B", "price": 5000, "quantity": 1}}
		book(t, p.event("payment.completed", "paid", fmt.Sprint("e", i), time.Now().UTC()))
	}
	owner := ucexport.Caller{Subject: uniq("accountant")}
	other := ucexport.Caller{Subject: "someone-else"}

	env := shared.GetEnv()
	shared.SetEnv(shared.Environment{ExportStorageDir: env.ExportStorageDir, ExportMaxRows: 5})
	_, err := uc.Export().RequestExport(ctx, owner, &exportdomain.RequestCreateExport{Type: "orders"})
	assert.Equal(t, http.StatusBadRequest, status(err), "more rows than ORDER_EXPORT_MAX_ROWS (5)")
	shared.SetEnv(env)

	job, err := uc.Export().RequestExport(ctx, owner, &exportdomain.RequestCreateExport{Type: "order_lines", Filter: json.RawMessage(`{"merchantId":"` + m5 + `"}`)})
	require.NoError(t, err)
	assert.Equal(t, "queued", job.Status)
	assert.EqualValues(t, 3, job.TotalRows)
	_, _, _, err = uc.Export().DownloadExport(ctx, owner, job.ID)
	assert.Equal(t, http.StatusConflict, status(err), "not ready yet")
	_, err = uc.Export().GetExport(ctx, other, job.ID)
	assert.Equal(t, http.StatusNotFound, status(err), "other people's exports are invisible")

	require.NoError(t, uc.Export().RunExport(ctx, job.ID))
	require.NoError(t, uc.Export().RunExport(ctx, job.ID), "a second run of a finished job is a no-op")
	job, err = uc.Export().GetExport(ctx, owner, job.ID)
	require.NoError(t, err)
	assert.Equal(t, "completed", job.Status)
	assert.EqualValues(t, 6, job.ProcessedRows)
	assert.NotNil(t, job.ExpiresAt)

	_, file, size, err := uc.Export().DownloadExport(ctx, owner, job.ID)
	require.NoError(t, err)
	body, _ := io.ReadAll(file)
	file.Close()
	assert.EqualValues(t, len(body), size)
	rows, err := csv.NewReader(strings.NewReader(string(body))).ReadAll()
	require.NoError(t, err)
	assert.Len(t, rows, 7, "header + 6 lines")
	assert.Equal(t, "item_name", rows[0][7])

	_, err = uc.Export().CancelExport(ctx, owner, job.ID)
	assert.Equal(t, http.StatusConflict, status(err), "a finished export cannot be cancelled")

	// a queued export cancelled before it runs never produces a file
	job2, err := uc.Export().RequestExport(ctx, owner, &exportdomain.RequestCreateExport{Type: "invoices", Filter: json.RawMessage(`{"merchantId":"` + m5 + `"}`)})
	require.NoError(t, err)
	job2, err = uc.Export().CancelExport(ctx, owner, job2.ID)
	require.NoError(t, err)
	assert.Equal(t, "cancelled", job2.Status)
	_, err = uc.Export().CancelExport(ctx, owner, job2.ID)
	assert.NoError(t, err, "cancelling twice is fine")
	require.NoError(t, uc.Export().RunExport(ctx, job2.ID))
	job2, _ = uc.Export().GetExport(ctx, owner, job2.ID)
	assert.Equal(t, "cancelled", job2.Status)
	assert.Empty(t, job2.FileName)

	// the sweeper queues again a job the queue lost, and fails it once out of attempts
	job3, err := uc.Export().RequestExport(ctx, owner, &exportdomain.RequestCreateExport{Type: "invoices", Filter: json.RawMessage(`{"merchantId":"` + m5 + `"}`)})
	require.NoError(t, err)
	_, err = db.Exec("UPDATE export_jobs SET updated_at = now() - interval '10 minutes', attempts = 3 WHERE id = $1", job3.ID)
	require.NoError(t, err)
	_, err = uc.Export().SweepExports(ctx)
	require.NoError(t, err)
	job3, _ = uc.Export().GetExport(ctx, owner, job3.ID)
	assert.Equal(t, "failed", job3.Status)

	// expiry: the file is purged and the download answers 410
	_, err = db.Exec("UPDATE export_jobs SET expires_at = now() - interval '1 minute' WHERE id = $1", job.ID)
	require.NoError(t, err)
	n, err := uc.Export().PurgeExports(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, 1)
	_, _, _, err = uc.Export().DownloadExport(ctx, owner, job.ID)
	assert.Equal(t, http.StatusGone, status(err))

	list, err := uc.Export().GetAllExports(ctx, owner, &exportdomain.FilterExport{Filter: candishared.Filter{Limit: 10, Page: 1}})
	require.NoError(t, err)
	assert.Len(t, list.Data, 3)
	assert.Equal(t, 1, activityCount(t, "order.export_requested", "export-"+job.ID))
}

func Test_Ledger_ConcurrentEventsOfOnePayment(t *testing.T) {
	m := uniq("MC")
	merchant(t, m)
	t0 := time.Now().UTC()
	for i := 0; i < 10; i++ {
		p := newPay(m, 10000)
		evs := []*orderdomain.PaymentEvent{
			p.event("payment.created", "pending", "", t0),
			p.event("payment.checkout_started", "processing", "x", t0.Add(time.Second)),
			p.event("payment.completed", "paid", "x", t0.Add(2*time.Second)),
			p.event("payment.completed", "paid", "x", t0.Add(2*time.Second)), // redelivered at once
		}
		errs := make(chan error, len(evs))
		for _, ev := range evs {
			go func(ev *orderdomain.PaymentEvent) {
				errs <- uc.Order().RecordPaymentEvent(ctx, ev, orderdomain.KafkaSource{Topic: ev.Event})
			}(ev)
		}
		for range evs {
			require.NoError(t, <-errs)
		}
		var orders, events, invoices int
		require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM orders WHERE payment_id = $1", p.id).Scan(&orders))
		require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM order_events e JOIN orders o ON o.id = e.order_id WHERE o.payment_id = $1", p.id).Scan(&events))
		require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM invoices i JOIN orders o ON o.id = i.order_id WHERE o.payment_id = $1", p.id).Scan(&invoices))
		assert.Equal(t, 1, orders, "one order per payment")
		assert.Equal(t, 3, events, "each event once")
		assert.Equal(t, 1, invoices, "one invoice")
		o := orderOf(t, p)
		assert.Equal(t, "paid", o.PaymentStatus)
		assert.Equal(t, "confirmed", o.OrderStatus)
	}
}
