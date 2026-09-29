package repository

import (
	"context"
	"time"

	"monorepo/globalshared/gormx"
	"monorepo/services/order/internal/modules/merchant/domain"
	shareddomain "monorepo/services/order/pkg/shared/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type merchantRepoSQL struct {
	readDB, writeDB *gorm.DB
}

// NewMerchantRepoSQL repo constructor
func NewMerchantRepoSQL(readDB, writeDB *gorm.DB) MerchantRepository {
	return &merchantRepoSQL{readDB: readDB, writeDB: writeDB}
}

func (r *merchantRepoSQL) db(ctx context.Context) *gorm.DB { return gormx.DB(ctx, r.writeDB) }

func (r *merchantRepoSQL) Find(ctx context.Context, merchantID string) (result shareddomain.MerchantSetting, err error) {
	err = r.db(ctx).Where("merchant_id = ?", merchantID).First(&result).Error
	return
}

func (r *merchantRepoSQL) filter(db *gorm.DB, f *domain.FilterMerchant) *gorm.DB {
	if f.Search != "" {
		db = db.Where("(merchant_id ILIKE ? OR name ILIKE ?)", gormx.Like(f.Search), gormx.Like(f.Search))
	}
	return db
}

func (r *merchantRepoSQL) FetchAll(ctx context.Context, f *domain.FilterMerchant) (result []shareddomain.MerchantSetting, err error) {
	db := r.filter(gormx.DB(ctx, r.readDB).Model(&shareddomain.MerchantSetting{}), f)
	err = gormx.ApplyPaging(db, &f.Filter, "merchant_id", "merchant_id", "name", "updated_at").Find(&result).Error
	return
}

func (r *merchantRepoSQL) Count(ctx context.Context, f *domain.FilterMerchant) int {
	var total int64
	r.filter(gormx.DB(ctx, r.readDB).Model(&shareddomain.MerchantSetting{}), f).Count(&total)
	return int(total)
}

func (r *merchantRepoSQL) Save(ctx context.Context, data *shareddomain.MerchantSetting) error {
	now := time.Now()
	data.UpdatedAt = now
	if data.CreatedAt.IsZero() {
		data.CreatedAt = now
	}
	return r.db(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "merchant_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "address", "tax_id", "order_prefix", "invoice_prefix", "credit_note_prefix", "tax_name", "tax_rate", "tax_mode", "rounding_mode", "rounding_unit", "timezone", "updated_at"}),
	}).Create(data).Error
}

func (r *merchantRepoSQL) Delete(ctx context.Context, merchantID string) (bool, error) {
	res := r.db(ctx).Where("merchant_id = ?", merchantID).Delete(&shareddomain.MerchantSetting{})
	return res.RowsAffected > 0, res.Error
}
