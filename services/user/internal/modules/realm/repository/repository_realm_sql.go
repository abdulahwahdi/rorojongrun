package repository

import (
	"context"

	"monorepo/services/user/internal/modules/realm/domain"
	"monorepo/services/user/pkg/helper"
	shareddomain "monorepo/services/user/pkg/shared/domain"

	"github.com/golangid/candi/tracer"
	"gorm.io/gorm"
)

type realmRepoSQL struct {
	readDB, writeDB *gorm.DB
}

// NewRealmRepoSQL constructor
func NewRealmRepoSQL(readDB, writeDB *gorm.DB) RealmRepository {
	return &realmRepoSQL{readDB: readDB, writeDB: writeDB}
}

func (r *realmRepoSQL) filter(db *gorm.DB, f *domain.FilterRealm) *gorm.DB {
	db = helper.Alive(db, "realms")
	if f.Name != "" {
		db = db.Where("name = ?", f.Name)
	}
	if f.Enabled != nil {
		db = db.Where("enabled = ?", *f.Enabled)
	}
	if f.Search != "" {
		db = db.Where("(name ILIKE ? OR display_name ILIKE ?)", helper.Like(f.Search), helper.Like(f.Search))
	}
	return db
}

func (r *realmRepoSQL) FetchAll(ctx context.Context, f *domain.FilterRealm) (data []shareddomain.Realm, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmRepoSQL:FetchAll")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	db := helper.ApplyPaging(r.filter(helper.DB(ctx, r.readDB), f), &f.Filter, "id", "id", "name", "created_at", "updated_at")
	err = db.Find(&data).Error
	return
}

func (r *realmRepoSQL) Count(ctx context.Context, f *domain.FilterRealm) int {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmRepoSQL:Count")
	defer trace.Finish()

	var total int64
	r.filter(helper.DB(ctx, r.readDB), f).Model(&shareddomain.Realm{}).Count(&total)
	return int(total)
}

func (r *realmRepoSQL) FindByID(ctx context.Context, id int) (res shareddomain.Realm, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmRepoSQL:FindByID")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = helper.Alive(helper.DB(ctx, r.readDB), "realms").Where("id = ?", id).First(&res).Error
	return
}

func (r *realmRepoSQL) FindByName(ctx context.Context, name string) (res shareddomain.Realm, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmRepoSQL:FindByName")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = helper.Alive(helper.DB(ctx, r.readDB), "realms").Where("name = ?", name).First(&res).Error
	return
}

func (r *realmRepoSQL) Save(ctx context.Context, data *shareddomain.Realm) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmRepoSQL:Save")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	return helper.Save(helper.DB(ctx, r.writeDB), data.ID, data)
}

func (r *realmRepoSQL) Delete(ctx context.Context, id int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmRepoSQL:Delete")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	return helper.SoftDelete(helper.DB(ctx, r.writeDB), &shareddomain.Realm{}, "id = ?", id)
}

type realmKeyRepoSQL struct {
	readDB, writeDB *gorm.DB
}

// NewRealmKeyRepoSQL constructor
func NewRealmKeyRepoSQL(readDB, writeDB *gorm.DB) RealmKeyRepository {
	return &realmKeyRepoSQL{readDB: readDB, writeDB: writeDB}
}

func (r *realmKeyRepoSQL) filter(db *gorm.DB, realmID int, f *domain.FilterRealmKey) *gorm.DB {
	db = db.Where("realm_id = ?", realmID)
	if f.Active != nil {
		db = db.Where("active = ?", *f.Active)
	}
	if f.Search != "" {
		db = db.Where("kid ILIKE ?", helper.Like(f.Search))
	}
	return db
}

func (r *realmKeyRepoSQL) FetchAll(ctx context.Context, realmID int, f *domain.FilterRealmKey) (data []shareddomain.RealmKey, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmKeyRepoSQL:FetchAll")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	db := helper.ApplyPaging(r.filter(helper.DB(ctx, r.readDB), realmID, f), &f.Filter, "id", "id", "created_at")
	err = db.Find(&data).Error
	return
}

func (r *realmKeyRepoSQL) Count(ctx context.Context, realmID int, f *domain.FilterRealmKey) int {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmKeyRepoSQL:Count")
	defer trace.Finish()

	var total int64
	r.filter(helper.DB(ctx, r.readDB), realmID, f).Model(&shareddomain.RealmKey{}).Count(&total)
	return int(total)
}

func (r *realmKeyRepoSQL) Find(ctx context.Context, realmID, id int) (res shareddomain.RealmKey, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmKeyRepoSQL:Find")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = helper.DB(ctx, r.readDB).Where("realm_id = ? AND id = ?", realmID, id).First(&res).Error
	return
}

func (r *realmKeyRepoSQL) FindByKID(ctx context.Context, kid string) (res shareddomain.RealmKey, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmKeyRepoSQL:FindByKID")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = helper.DB(ctx, r.readDB).Where("kid = ?", kid).First(&res).Error
	return
}

func (r *realmKeyRepoSQL) FindActive(ctx context.Context, realmID int) (res shareddomain.RealmKey, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmKeyRepoSQL:FindActive")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = helper.DB(ctx, r.readDB).Where("realm_id = ? AND active = TRUE", realmID).Order("id DESC").First(&res).Error
	return
}

func (r *realmKeyRepoSQL) FetchActive(ctx context.Context, realmID int) (data []shareddomain.RealmKey, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmKeyRepoSQL:FetchActive")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = helper.DB(ctx, r.readDB).Where("realm_id = ? AND active = TRUE", realmID).Order("id DESC").Find(&data).Error
	return
}

func (r *realmKeyRepoSQL) Save(ctx context.Context, data *shareddomain.RealmKey) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmKeyRepoSQL:Save")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	return helper.Save(helper.DB(ctx, r.writeDB), data.ID, data)
}

func (r *realmKeyRepoSQL) Delete(ctx context.Context, realmID, id int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmKeyRepoSQL:Delete")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	return helper.DB(ctx, r.writeDB).Where("realm_id = ? AND id = ?", realmID, id).Delete(&shareddomain.RealmKey{}).Error
}
