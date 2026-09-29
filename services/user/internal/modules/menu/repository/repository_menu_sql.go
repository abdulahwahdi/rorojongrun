package repository

import (
	"context"

	"monorepo/services/user/internal/modules/menu/domain"
	"monorepo/services/user/pkg/helper"
	shareddomain "monorepo/services/user/pkg/shared/domain"

	"github.com/golangid/candi/tracer"
	"gorm.io/gorm"
)

type menuRepoSQL struct {
	readDB, writeDB *gorm.DB
}

// NewMenuRepoSQL constructor
func NewMenuRepoSQL(readDB, writeDB *gorm.DB) MenuRepository {
	return &menuRepoSQL{readDB: readDB, writeDB: writeDB}
}

func (r *menuRepoSQL) filter(db *gorm.DB, clientID int, f *domain.FilterMenu) *gorm.DB {
	db = helper.Alive(db, "menus").Where("client_id = ?", clientID)
	if f.ParentID != nil {
		db = db.Where("parent_id = ?", *f.ParentID)
	}
	if f.Search != "" {
		db = db.Where("(key ILIKE ? OR label ILIKE ?)", helper.Like(f.Search), helper.Like(f.Search))
	}
	return db
}

func (r *menuRepoSQL) FetchAll(ctx context.Context, clientID int, f *domain.FilterMenu) (data []shareddomain.Menu, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "MenuRepoSQL:FetchAll")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	if f.OrderBy == "" {
		f.OrderBy, f.Sort = "sort_order", "asc"
	}
	db := helper.ApplyPaging(r.filter(helper.DB(ctx, r.readDB), clientID, f), &f.Filter, "sort_order", "id", "key", "label", "sort_order", "created_at")
	err = db.Find(&data).Error
	return
}

func (r *menuRepoSQL) Count(ctx context.Context, clientID int, f *domain.FilterMenu) int {
	trace, ctx := tracer.StartTraceWithContext(ctx, "MenuRepoSQL:Count")
	defer trace.Finish()

	var total int64
	r.filter(helper.DB(ctx, r.readDB), clientID, f).Model(&shareddomain.Menu{}).Count(&total)
	return int(total)
}

func (r *menuRepoSQL) FetchAllOfClient(ctx context.Context, clientID int) (data []shareddomain.Menu, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "MenuRepoSQL:FetchAllOfClient")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = helper.Alive(helper.DB(ctx, r.readDB), "menus").Where("client_id = ?", clientID).Order("sort_order, id").Find(&data).Error
	return
}

func (r *menuRepoSQL) Find(ctx context.Context, clientID, id int) (res shareddomain.Menu, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "MenuRepoSQL:Find")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = helper.Alive(helper.DB(ctx, r.readDB), "menus").Where("client_id = ? AND id = ?", clientID, id).First(&res).Error
	return
}

func (r *menuRepoSQL) FindByKey(ctx context.Context, clientID int, key string) (res shareddomain.Menu, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "MenuRepoSQL:FindByKey")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = helper.Alive(helper.DB(ctx, r.readDB), "menus").Where("client_id = ? AND key = ?", clientID, key).First(&res).Error
	return
}

func (r *menuRepoSQL) Save(ctx context.Context, data *shareddomain.Menu) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "MenuRepoSQL:Save")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	return helper.MapDBError(helper.Save(helper.DB(ctx, r.writeDB), data.ID, data))
}

func (r *menuRepoSQL) DeleteMany(ctx context.Context, ids []int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "MenuRepoSQL:DeleteMany")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	if len(ids) == 0 {
		return nil
	}
	return helper.SoftDelete(helper.DB(ctx, r.writeDB), &shareddomain.Menu{}, "id IN ?", ids)
}

func (r *menuRepoSQL) DeleteByClient(ctx context.Context, clientID int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "MenuRepoSQL:DeleteByClient")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	return helper.SoftDelete(helper.DB(ctx, r.writeDB), &shareddomain.Menu{}, "client_id = ?", clientID)
}
