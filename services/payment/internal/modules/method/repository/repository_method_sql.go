package repository

import (
	"context"
	"time"

	"monorepo/services/payment/internal/modules/method/domain"
	"monorepo/services/payment/pkg/helper"
	shareddomain "monorepo/services/payment/pkg/shared/domain"

	"github.com/golangid/candi/tracer"
	"gorm.io/gorm"
)

type methodRepoSQL struct {
	readDB, writeDB *gorm.DB
}

// NewMethodRepoSQL repo constructor. Reads use the write connection so admin changes are
// visible immediately to checkout.
func NewMethodRepoSQL(readDB, writeDB *gorm.DB) MethodRepository {
	return &methodRepoSQL{readDB: readDB, writeDB: writeDB}
}

func (r *methodRepoSQL) filter(db *gorm.DB, f *domain.FilterMethod) *gorm.DB {
	if f.Type != "" {
		db = db.Where("type = ?", f.Type)
	}
	if f.GatewayCode != "" {
		db = db.Where("gateway_code = ?", f.GatewayCode)
	}
	if f.IsEnabled != nil {
		db = db.Where("is_enabled = ?", *f.IsEnabled)
	}
	if f.Search != "" {
		db = db.Where("(code ILIKE ? OR name ILIKE ?)", helper.Like(f.Search), helper.Like(f.Search))
	}
	return db
}

func (r *methodRepoSQL) FetchAll(ctx context.Context, f *domain.FilterMethod) (data []shareddomain.Method, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "MethodRepoSQL:FetchAll")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	db := r.filter(helper.DB(ctx, r.writeDB), f)
	if f.OrderBy == "" {
		f.OrderBy, f.Sort = "sort_order", "asc"
	}
	err = helper.ApplyPaging(db, &f.Filter, "sort_order", "code", "name", "type", "created_at", "updated_at").Find(&data).Error
	return
}

func (r *methodRepoSQL) Count(ctx context.Context, f *domain.FilterMethod) (count int) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "MethodRepoSQL:Count")
	defer trace.Finish()

	var total int64
	r.filter(helper.DB(ctx, r.writeDB), f).Model(&shareddomain.Method{}).Count(&total)
	return int(total)
}

func (r *methodRepoSQL) FindByID(ctx context.Context, id int) (result shareddomain.Method, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "MethodRepoSQL:FindByID")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = helper.DB(ctx, r.writeDB).Where("id = ?", id).First(&result).Error
	return
}

func (r *methodRepoSQL) FindByCode(ctx context.Context, code string) (result shareddomain.Method, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "MethodRepoSQL:FindByCode")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = helper.DB(ctx, r.writeDB).Where("code = ?", code).First(&result).Error
	return
}

func (r *methodRepoSQL) FetchEnabled(ctx context.Context) (data []shareddomain.Method, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "MethodRepoSQL:FetchEnabled")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = helper.DB(ctx, r.writeDB).Where("is_enabled = true").Order("sort_order ASC, id ASC").Find(&data).Error
	return
}

func (r *methodRepoSQL) Save(ctx context.Context, data *shareddomain.Method) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "MethodRepoSQL:Save")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	now := time.Now()
	data.UpdatedAt = now
	db := helper.DB(ctx, r.writeDB)
	if data.ID == 0 {
		data.CreatedAt = now
		return db.Create(data).Error
	}
	return db.Model(data).Select("*").Omit("id", "created_at").Updates(data).Error
}

func (r *methodRepoSQL) Delete(ctx context.Context, id int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "MethodRepoSQL:Delete")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	return helper.DB(ctx, r.writeDB).Where("id = ?", id).Delete(&shareddomain.Method{}).Error
}
