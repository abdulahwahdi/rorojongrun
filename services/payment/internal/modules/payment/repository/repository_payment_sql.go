package repository

import (
	"context"
	"time"

	"monorepo/globalshared/gormx"
	"monorepo/services/payment/internal/modules/payment/domain"
	shareddomain "monorepo/services/payment/pkg/shared/domain"

	"github.com/golangid/candi/tracer"
	"gorm.io/gorm"
)

type paymentRepoSQL struct {
	readDB, writeDB *gorm.DB
}

// NewPaymentRepoSQL repo constructor. Every read uses the write connection: payment state
// is read right after it is written (and locked) so replica lag is not acceptable.
func NewPaymentRepoSQL(readDB, writeDB *gorm.DB) PaymentRepository {
	return &paymentRepoSQL{readDB: readDB, writeDB: writeDB}
}

func (r *paymentRepoSQL) db(ctx context.Context) *gorm.DB { return gormx.DB(ctx, r.writeDB) }

// save inserts when create is true, otherwise writes every column but the keys
func (r *paymentRepoSQL) save(ctx context.Context, model any, create bool) error {
	if create {
		return r.db(ctx).Create(model).Error
	}
	return r.db(ctx).Model(model).Select("*").Omit("id", "created_at").Updates(model).Error
}

// ---- payments

func (r *paymentRepoSQL) SavePayment(ctx context.Context, data *shareddomain.Payment) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PaymentRepoSQL:SavePayment")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	now := time.Now()
	create := data.CreatedAt.IsZero()
	if create {
		data.CreatedAt = now
	}
	data.UpdatedAt = now
	return r.save(ctx, data, create)
}

func (r *paymentRepoSQL) findPayment(ctx context.Context, where string, arg any) (result shareddomain.Payment, err error) {
	err = r.db(ctx).Where(where, arg).First(&result).Error
	return
}

func (r *paymentRepoSQL) FindPaymentByID(ctx context.Context, id string) (shareddomain.Payment, error) {
	return r.findPayment(ctx, "id = ?", id)
}

func (r *paymentRepoSQL) FindPaymentByToken(ctx context.Context, token string) (shareddomain.Payment, error) {
	return r.findPayment(ctx, "token = ?", token)
}

func (r *paymentRepoSQL) FindActivePaymentByRef(ctx context.Context, source, referenceID string) (result shareddomain.Payment, err error) {
	err = r.db(ctx).Where("source = ? AND reference_id = ? AND status IN ?", source, referenceID,
		[]string{shareddomain.PaymentPending, shareddomain.PaymentProcessing}).First(&result).Error
	return
}

func (r *paymentRepoSQL) LockPayment(ctx context.Context, id string) (result shareddomain.Payment, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PaymentRepoSQL:LockPayment")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = gormx.ForUpdate(r.db(ctx)).Where("id = ?", id).First(&result).Error
	return
}

func (r *paymentRepoSQL) filterPayments(db *gorm.DB, f *domain.FilterPayment) *gorm.DB {
	if f.Source != "" {
		db = db.Where("source = ?", f.Source)
	}
	if f.ReferenceID != "" {
		db = db.Where("reference_id = ?", f.ReferenceID)
	}
	if f.Status != "" {
		db = db.Where("status = ?", f.Status)
	}
	if f.MethodCode != "" {
		db = db.Where("method_code = ?", f.MethodCode)
	}
	if f.StartDate != "" {
		db = db.Where("created_at >= ?", f.StartDate)
	}
	if f.EndDate != "" {
		db = db.Where("created_at < ?", f.EndDate)
	}
	if f.Search != "" {
		db = db.Where("(reference_id ILIKE ? OR description ILIKE ?)", gormx.Like(f.Search), gormx.Like(f.Search))
	}
	return db
}

func (r *paymentRepoSQL) FetchAllPayments(ctx context.Context, f *domain.FilterPayment) (data []shareddomain.Payment, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PaymentRepoSQL:FetchAllPayments")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	db := r.filterPayments(r.db(ctx), f)
	err = gormx.ApplyPaging(db, &f.Filter, "created_at", "created_at", "updated_at", "amount", "status").Find(&data).Error
	return
}

func (r *paymentRepoSQL) CountPayments(ctx context.Context, f *domain.FilterPayment) int {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PaymentRepoSQL:CountPayments")
	defer trace.Finish()

	var total int64
	r.filterPayments(r.db(ctx), f).Model(&shareddomain.Payment{}).Count(&total)
	return int(total)
}

func (r *paymentRepoSQL) FetchExpiredPaymentIDs(ctx context.Context, now time.Time, limit int) (ids []string, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PaymentRepoSQL:FetchExpiredPaymentIDs")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = r.db(ctx).Model(&shareddomain.Payment{}).
		Where("status IN ? AND expires_at < ?", []string{shareddomain.PaymentPending, shareddomain.PaymentProcessing}, now).
		Order("expires_at ASC").Limit(limit).Pluck("id", &ids).Error
	return
}

// ---- transactions

func (r *paymentRepoSQL) SaveTransaction(ctx context.Context, data *shareddomain.Transaction) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PaymentRepoSQL:SaveTransaction")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	now := time.Now()
	create := data.CreatedAt.IsZero()
	if create {
		data.CreatedAt = now
	}
	data.UpdatedAt = now
	return r.save(ctx, data, create)
}

func (r *paymentRepoSQL) FindTransactionByID(ctx context.Context, id string) (result shareddomain.Transaction, err error) {
	err = r.db(ctx).Where("id = ?", id).First(&result).Error
	return
}

func (r *paymentRepoSQL) FindTransactionByCashCode(ctx context.Context, cashCode string) (result shareddomain.Transaction, err error) {
	err = r.db(ctx).Where("cash_code = ?", cashCode).First(&result).Error
	return
}

func (r *paymentRepoSQL) FetchTransactionsByPayment(ctx context.Context, paymentID string) (data []shareddomain.Transaction, err error) {
	err = r.db(ctx).Where("payment_id = ?", paymentID).Order("created_at ASC").Find(&data).Error
	return
}

func (r *paymentRepoSQL) FindOpenTransaction(ctx context.Context, paymentID string) (result shareddomain.Transaction, err error) {
	err = r.db(ctx).Where("payment_id = ? AND status = ?", paymentID, shareddomain.TransactionPending).
		Order("created_at DESC").First(&result).Error
	return
}

// ---- callback logs

func (r *paymentRepoSQL) SaveCallbackLog(ctx context.Context, data *shareddomain.CallbackLog) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PaymentRepoSQL:SaveCallbackLog")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	now := time.Now()
	data.UpdatedAt = now
	if data.ID == 0 {
		data.CreatedAt = now
		return r.db(ctx).Create(data).Error
	}
	return r.db(ctx).Model(data).Select("*").Omit("id", "created_at").Updates(data).Error
}

func (r *paymentRepoSQL) FindCallbackLogByID(ctx context.Context, id int) (result shareddomain.CallbackLog, err error) {
	err = r.db(ctx).Where("id = ?", id).First(&result).Error
	return
}

func (r *paymentRepoSQL) filterLogs(db *gorm.DB, f *domain.FilterCallbackLog) *gorm.DB {
	if f.GatewayCode != "" {
		db = db.Where("gateway_code = ?", f.GatewayCode)
	}
	if f.Status != "" {
		db = db.Where("status = ?", f.Status)
	}
	if f.ExternalID != "" {
		db = db.Where("external_id = ?", f.ExternalID)
	}
	if f.Topic != "" {
		db = db.Where("topic = ?", f.Topic)
	}
	return db
}

func (r *paymentRepoSQL) FetchAllCallbackLogs(ctx context.Context, f *domain.FilterCallbackLog) (data []shareddomain.CallbackLog, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PaymentRepoSQL:FetchAllCallbackLogs")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	db := r.filterLogs(r.db(ctx), f)
	err = gormx.ApplyPaging(db, &f.Filter, "id", "id", "created_at", "status").Find(&data).Error
	return
}

func (r *paymentRepoSQL) CountCallbackLogs(ctx context.Context, f *domain.FilterCallbackLog) int {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PaymentRepoSQL:CountCallbackLogs")
	defer trace.Finish()

	var total int64
	r.filterLogs(r.db(ctx), f).Model(&shareddomain.CallbackLog{}).Count(&total)
	return int(total)
}

// ---- outbox

func (r *paymentRepoSQL) SaveOutbox(ctx context.Context, data *shareddomain.Outbox) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PaymentRepoSQL:SaveOutbox")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	now := time.Now()
	data.UpdatedAt = now
	if data.ID == 0 {
		data.CreatedAt = now
		return r.db(ctx).Create(data).Error
	}
	return r.db(ctx).Model(data).Select("*").Omit("id", "created_at").Updates(data).Error
}

func (r *paymentRepoSQL) LockPendingOutbox(ctx context.Context, limit int) (data []shareddomain.Outbox, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PaymentRepoSQL:LockPendingOutbox")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = gormx.ForUpdateSkipLocked(r.db(ctx)).Where("published_at IS NULL").Order("id ASC").Limit(limit).Find(&data).Error
	return
}
