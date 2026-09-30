package repository

import (
	"context"
	"strings"
	"time"

	"monorepo/globalshared/gormx"
	shareddomain "monorepo/services/order/pkg/shared/domain"

	"github.com/golangid/candi/tracer"
	"gorm.io/gorm"
)

type invoiceRepoSQL struct {
	readDB, writeDB *gorm.DB
}

// NewInvoiceRepoSQL repo constructor
func NewInvoiceRepoSQL(readDB, writeDB *gorm.DB) InvoiceRepository {
	return &invoiceRepoSQL{readDB: readDB, writeDB: writeDB}
}

func (r *invoiceRepoSQL) db(ctx context.Context) *gorm.DB   { return gormx.DB(ctx, r.writeDB) }
func (r *invoiceRepoSQL) read(ctx context.Context) *gorm.DB { return gormx.DB(ctx, r.readDB) }

func (r *invoiceRepoSQL) Create(ctx context.Context, data *shareddomain.Invoice) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "InvoiceRepoSQL:Create")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	now := time.Now()
	data.CreatedAt, data.UpdatedAt = now, now
	if err = r.db(ctx).Omit("Items").Create(data).Error; err != nil || len(data.Items) == 0 {
		return err
	}
	for i := range data.Items {
		data.Items[i].InvoiceID = data.ID
	}
	return r.db(ctx).Create(&data.Items).Error
}

func (r *invoiceRepoSQL) Update(ctx context.Context, data *shareddomain.Invoice) error {
	data.UpdatedAt = time.Now()
	return r.db(ctx).Model(data).Select("status", "emailed_at", "updated_at").Updates(data).Error
}

func (r *invoiceRepoSQL) FindByID(ctx context.Context, id int64) (result shareddomain.Invoice, err error) {
	err = r.db(ctx).Where("id = ?", id).First(&result).Error
	return
}

func (r *invoiceRepoSQL) FindByNumber(ctx context.Context, number string) (result shareddomain.Invoice, err error) {
	err = r.db(ctx).Where("number = ?", number).First(&result).Error
	return
}

func (r *invoiceRepoSQL) FindByOrder(ctx context.Context, orderID int64, invoiceType string) (result shareddomain.Invoice, err error) {
	err = r.db(ctx).Where("order_id = ? AND type = ?", orderID, invoiceType).First(&result).Error
	return
}

func (r *invoiceRepoSQL) FetchByOrder(ctx context.Context, orderID int64) (result []shareddomain.Invoice, err error) {
	err = r.db(ctx).Where("order_id = ?", orderID).Order("id").Find(&result).Error
	return
}

func (r *invoiceRepoSQL) FetchItems(ctx context.Context, invoiceIDs ...int64) (result []shareddomain.InvoiceItem, err error) {
	if len(invoiceIDs) == 0 {
		return nil, nil
	}
	err = r.db(ctx).Where("invoice_id IN ?", invoiceIDs).Order("invoice_id, line_no").Find(&result).Error
	return
}

func filterInvoices(db *gorm.DB, f *shareddomain.FilterInvoice) *gorm.DB {
	for col, v := range map[string]string{"type": f.Type, "status": f.Status, "merchant_id": f.MerchantID, "outlet_id": f.OutletID} {
		if v != "" {
			db = db.Where(col+" = ?", v)
		}
	}
	if f.OrderID > 0 {
		db = db.Where("order_id = ?", f.OrderID)
	}
	if f.From != nil {
		db = db.Where("issued_at >= ?", *f.From)
	}
	if f.To != nil {
		db = db.Where("issued_at < ?", *f.To)
	}
	if s := strings.TrimSpace(f.Search); s != "" {
		like := gormx.Like(s)
		db = db.Where("(number ILIKE ? OR order_number ILIKE ? OR customer_name ILIKE ? OR customer_email ILIKE ?)", like, like, like, like)
	}
	return db
}

func (r *invoiceRepoSQL) FetchAll(ctx context.Context, f *shareddomain.FilterInvoice) (result []shareddomain.Invoice, err error) {
	db := filterInvoices(r.read(ctx).Model(&shareddomain.Invoice{}), f)
	err = gormx.ApplyPaging(db, &f.Filter, "issued_at", "issued_at", "number", "total_amount").Find(&result).Error
	return
}

func (r *invoiceRepoSQL) Count(ctx context.Context, f *shareddomain.FilterInvoice) int {
	var total int64
	filterInvoices(r.read(ctx).Model(&shareddomain.Invoice{}), f).Count(&total)
	return int(total)
}

func (r *invoiceRepoSQL) FetchAfter(ctx context.Context, f *shareddomain.FilterInvoice, afterID int64, limit int) (result []shareddomain.Invoice, err error) {
	err = filterInvoices(r.read(ctx).Model(&shareddomain.Invoice{}), f).
		Where("id > ?", afterID).Order("id").Limit(limit).Find(&result).Error
	return
}
