package repository

import (
	"context"
	"time"

	"monorepo/globalshared/gormx"
	"monorepo/services/order/internal/modules/shift/domain"
	shareddomain "monorepo/services/order/pkg/shared/domain"

	"gorm.io/gorm"
)

type shiftRepoSQL struct {
	readDB, writeDB *gorm.DB
}

// NewShiftRepoSQL repo constructor
func NewShiftRepoSQL(readDB, writeDB *gorm.DB) ShiftRepository {
	return &shiftRepoSQL{readDB: readDB, writeDB: writeDB}
}

func (r *shiftRepoSQL) db(ctx context.Context) *gorm.DB { return gormx.DB(ctx, r.writeDB) }

func (r *shiftRepoSQL) Create(ctx context.Context, data *shareddomain.CashShift) error {
	now := time.Now()
	data.CreatedAt, data.UpdatedAt = now, now
	return r.db(ctx).Create(data).Error
}

func (r *shiftRepoSQL) Update(ctx context.Context, data *shareddomain.CashShift) error {
	data.UpdatedAt = time.Now()
	return r.db(ctx).Model(data).Select("*").Omit("id", "created_at").Updates(data).Error
}

func (r *shiftRepoSQL) FindByID(ctx context.Context, id int64) (result shareddomain.CashShift, err error) {
	err = r.db(ctx).Where("id = ?", id).First(&result).Error
	return
}

func (r *shiftRepoSQL) LockByID(ctx context.Context, id int64) (result shareddomain.CashShift, err error) {
	err = gormx.ForUpdate(r.db(ctx)).Where("id = ?", id).First(&result).Error
	return
}

func (r *shiftRepoSQL) FindOpen(ctx context.Context, merchantID, outletID, cashierID string) (result shareddomain.CashShift, err error) {
	err = r.db(ctx).Where("merchant_id = ? AND outlet_id = ? AND cashier_id = ? AND status = ?",
		merchantID, outletID, cashierID, shareddomain.ShiftOpen).First(&result).Error
	return
}

func (r *shiftRepoSQL) filter(db *gorm.DB, f *domain.FilterShift) *gorm.DB {
	for col, v := range map[string]string{"merchant_id": f.MerchantID, "outlet_id": f.OutletID, "cashier_id": f.CashierID, "status": f.Status} {
		if v != "" {
			db = db.Where(col+" = ?", v)
		}
	}
	return db
}

func (r *shiftRepoSQL) FetchAll(ctx context.Context, f *domain.FilterShift) (result []shareddomain.CashShift, err error) {
	db := r.filter(gormx.DB(ctx, r.readDB).Model(&shareddomain.CashShift{}), f)
	err = gormx.ApplyPaging(db, &f.Filter, "opened_at", "opened_at", "closed_at").Find(&result).Error
	return
}

func (r *shiftRepoSQL) Count(ctx context.Context, f *domain.FilterShift) int {
	var total int64
	r.filter(gormx.DB(ctx, r.readDB).Model(&shareddomain.CashShift{}), f).Count(&total)
	return int(total)
}
