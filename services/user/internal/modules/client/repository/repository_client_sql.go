package repository

import (
	"context"

	"monorepo/globalshared/gormx"
	"monorepo/globalshared/rest"
	"monorepo/services/user/internal/modules/client/domain"
	"monorepo/services/user/pkg/helper"
	shareddomain "monorepo/services/user/pkg/shared/domain"

	"github.com/golangid/candi/tracer"
	"gorm.io/gorm"
)

type clientRepoSQL struct {
	readDB, writeDB *gorm.DB
}

// NewClientRepoSQL constructor
func NewClientRepoSQL(readDB, writeDB *gorm.DB) ClientRepository {
	return &clientRepoSQL{readDB: readDB, writeDB: writeDB}
}

func (r *clientRepoSQL) filter(db *gorm.DB, realmID int, f *domain.FilterClient) *gorm.DB {
	db = helper.Alive(db, "clients").Where("realm_id = ?", realmID)
	if f.ClientID != "" {
		db = db.Where("client_id = ?", f.ClientID)
	}
	if f.Type != "" {
		db = db.Where("type = ?", f.Type)
	}
	if f.Enabled != nil {
		db = db.Where("enabled = ?", *f.Enabled)
	}
	if f.Search != "" {
		db = db.Where("(client_id ILIKE ? OR name ILIKE ?)", gormx.Like(f.Search), gormx.Like(f.Search))
	}
	return db
}

func (r *clientRepoSQL) FetchAll(ctx context.Context, realmID int, f *domain.FilterClient) (data []shareddomain.Client, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "ClientRepoSQL:FetchAll")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	db := gormx.ApplyPaging(r.filter(gormx.DB(ctx, r.readDB), realmID, f), &f.Filter, "id", "id", "client_id", "name", "created_at", "updated_at")
	err = db.Find(&data).Error
	return
}

func (r *clientRepoSQL) Count(ctx context.Context, realmID int, f *domain.FilterClient) int {
	trace, ctx := tracer.StartTraceWithContext(ctx, "ClientRepoSQL:Count")
	defer trace.Finish()

	var total int64
	r.filter(gormx.DB(ctx, r.readDB), realmID, f).Model(&shareddomain.Client{}).Count(&total)
	return int(total)
}

func (r *clientRepoSQL) Find(ctx context.Context, realmID, id int) (res shareddomain.Client, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "ClientRepoSQL:Find")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = helper.Alive(gormx.DB(ctx, r.readDB), "clients").Where("realm_id = ? AND id = ?", realmID, id).First(&res).Error
	return
}

func (r *clientRepoSQL) FindByClientID(ctx context.Context, realmID int, clientID string) (res shareddomain.Client, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "ClientRepoSQL:FindByClientID")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = helper.Alive(gormx.DB(ctx, r.readDB), "clients").Where("realm_id = ? AND client_id = ?", realmID, clientID).First(&res).Error
	return
}

func (r *clientRepoSQL) Save(ctx context.Context, data *shareddomain.Client) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "ClientRepoSQL:Save")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	return rest.MapDBError(helper.Save(gormx.DB(ctx, r.writeDB), data.ID, data))
}

func (r *clientRepoSQL) Delete(ctx context.Context, id int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "ClientRepoSQL:Delete")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	return helper.SoftDelete(gormx.DB(ctx, r.writeDB), &shareddomain.Client{}, "id = ?", id)
}
