package repository

import (
	"context"
	"time"

	"monorepo/globalshared/gormx"
	"monorepo/services/user/internal/modules/auth/domain"
	"monorepo/services/user/pkg/helper"
	shareddomain "monorepo/services/user/pkg/shared/domain"

	"github.com/golangid/candi/tracer"
	"gorm.io/gorm"
)

type sessionRepoSQL struct {
	readDB, writeDB *gorm.DB
}

// NewSessionRepoSQL constructor
func NewSessionRepoSQL(readDB, writeDB *gorm.DB) SessionRepository {
	return &sessionRepoSQL{readDB: readDB, writeDB: writeDB}
}

func (r *sessionRepoSQL) filter(db *gorm.DB, realmID int, f *domain.FilterSession) *gorm.DB {
	db = db.Where("realm_id = ?", realmID)
	if f.UserID != nil {
		db = db.Where("user_id = ?", *f.UserID)
	}
	if f.ActiveOnly {
		db = db.Where("revoked_at IS NULL AND rotated_at IS NULL AND expires_at > ?", time.Now())
	}
	if f.Search != "" {
		db = db.Where("(ip ILIKE ? OR user_agent ILIKE ?)", gormx.Like(f.Search), gormx.Like(f.Search))
	}
	return db
}

func (r *sessionRepoSQL) FetchAll(ctx context.Context, realmID int, f *domain.FilterSession) (data []shareddomain.Session, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "SessionRepoSQL:FetchAll")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	db := gormx.ApplyPaging(r.filter(gormx.DB(ctx, r.readDB), realmID, f), &f.Filter, "id", "id", "user_id", "created_at", "expires_at")
	err = db.Find(&data).Error
	return
}

func (r *sessionRepoSQL) Count(ctx context.Context, realmID int, f *domain.FilterSession) int {
	trace, ctx := tracer.StartTraceWithContext(ctx, "SessionRepoSQL:Count")
	defer trace.Finish()

	var total int64
	r.filter(gormx.DB(ctx, r.readDB), realmID, f).Model(&shareddomain.Session{}).Count(&total)
	return int(total)
}

func (r *sessionRepoSQL) Find(ctx context.Context, realmID, id int) (res shareddomain.Session, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "SessionRepoSQL:Find")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = gormx.DB(ctx, r.readDB).Where("realm_id = ? AND id = ?", realmID, id).First(&res).Error
	return
}

func (r *sessionRepoSQL) FindByRefreshHash(ctx context.Context, hash string) (res shareddomain.Session, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "SessionRepoSQL:FindByRefreshHash")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = gormx.DB(ctx, r.readDB).Where("refresh_hash = ?", hash).First(&res).Error
	return
}

func (r *sessionRepoSQL) Save(ctx context.Context, data *shareddomain.Session) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "SessionRepoSQL:Save")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	db := gormx.DB(ctx, r.writeDB)
	if data.ID != 0 {
		return helper.Save(db, data.ID, data)
	}
	if err = db.Create(data).Error; err != nil {
		return err
	}
	if data.FamilyID == 0 {
		data.FamilyID = data.ID
		return db.Model(&shareddomain.Session{}).Where("id = ?", data.ID).Update("family_id", data.ID).Error
	}
	return nil
}

func (r *sessionRepoSQL) MarkRotated(ctx context.Context, id int) (ok bool, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "SessionRepoSQL:MarkRotated")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	res := gormx.DB(ctx, r.writeDB).Model(&shareddomain.Session{}).
		Where("id = ? AND rotated_at IS NULL AND revoked_at IS NULL", id).Update("rotated_at", time.Now())
	return res.RowsAffected == 1, res.Error
}

func (r *sessionRepoSQL) IsFamilyActive(ctx context.Context, familyID int) (active bool, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "SessionRepoSQL:IsFamilyActive")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	var total, revoked int64
	err = gormx.DB(ctx, r.readDB).Model(&shareddomain.Session{}).Where("family_id = ?", familyID).
		Select("COUNT(*), COUNT(revoked_at)").Row().Scan(&total, &revoked)
	return total > 0 && revoked == 0, err
}

func (r *sessionRepoSQL) revoke(ctx context.Context, name, where string, arg any) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, name)
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	return gormx.DB(ctx, r.writeDB).Model(&shareddomain.Session{}).
		Where(where+" AND revoked_at IS NULL", arg).Update("revoked_at", time.Now()).Error
}

func (r *sessionRepoSQL) RevokeFamily(ctx context.Context, familyID int) error {
	return r.revoke(ctx, "SessionRepoSQL:RevokeFamily", "family_id = ?", familyID)
}

func (r *sessionRepoSQL) RevokeByUser(ctx context.Context, userID int) error {
	return r.revoke(ctx, "SessionRepoSQL:RevokeByUser", "user_id = ?", userID)
}

func (r *sessionRepoSQL) RevokeByClient(ctx context.Context, clientID int) error {
	return r.revoke(ctx, "SessionRepoSQL:RevokeByClient", "client_id = ?", clientID)
}

func (r *sessionRepoSQL) RevokeByRealm(ctx context.Context, realmID int) error {
	return r.revoke(ctx, "SessionRepoSQL:RevokeByRealm", "realm_id = ?", realmID)
}
