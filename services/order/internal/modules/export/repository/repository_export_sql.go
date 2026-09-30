package repository

import (
	"context"
	"time"

	"monorepo/globalshared/gormx"
	"monorepo/services/order/internal/modules/export/domain"
	shareddomain "monorepo/services/order/pkg/shared/domain"

	"gorm.io/gorm"
)

type exportRepoSQL struct {
	readDB, writeDB *gorm.DB
}

// NewExportRepoSQL repo constructor. Job state is read on the write connection: the worker and
// the API race on it (cancel, claim).
func NewExportRepoSQL(readDB, writeDB *gorm.DB) ExportRepository {
	return &exportRepoSQL{readDB: readDB, writeDB: writeDB}
}

func (r *exportRepoSQL) db(ctx context.Context) *gorm.DB { return gormx.DB(ctx, r.writeDB) }

func (r *exportRepoSQL) Create(ctx context.Context, data *shareddomain.ExportJob) error {
	now := time.Now()
	data.CreatedAt, data.UpdatedAt = now, now
	return r.db(ctx).Create(data).Error
}

func (r *exportRepoSQL) Update(ctx context.Context, data *shareddomain.ExportJob) error {
	data.UpdatedAt = time.Now()
	return r.db(ctx).Model(data).Select("*").Omit("id", "created_at").Updates(data).Error
}

func (r *exportRepoSQL) FindByID(ctx context.Context, id string) (result shareddomain.ExportJob, err error) {
	err = r.db(ctx).Where("id = ?", id).First(&result).Error
	return
}

func (r *exportRepoSQL) LockByID(ctx context.Context, id string) (result shareddomain.ExportJob, err error) {
	err = gormx.ForUpdate(r.db(ctx)).Where("id = ?", id).First(&result).Error
	return
}

func (r *exportRepoSQL) Claim(ctx context.Context, id string, now time.Time) (bool, error) {
	res := r.db(ctx).Model(&shareddomain.ExportJob{}).
		Where("id = ? AND status = ? AND NOT cancel_requested", id, shareddomain.ExportQueued).
		Updates(map[string]any{
			"status": shareddomain.ExportRunning, "attempts": gorm.Expr("attempts + 1"), "started_at": now,
			"heartbeat_at": now, "processed_rows": 0, "error": "", "updated_at": now,
		})
	return res.RowsAffected > 0, res.Error
}

func (r *exportRepoSQL) Progress(ctx context.Context, id string, processed int64, now time.Time) (bool, error) {
	var job shareddomain.ExportJob
	err := r.db(ctx).Model(&job).Where("id = ?", id).
		Updates(map[string]any{"processed_rows": processed, "heartbeat_at": now, "updated_at": now}).Error
	if err != nil {
		return false, err
	}
	err = r.db(ctx).Select("cancel_requested", "status").Where("id = ?", id).First(&job).Error
	return job.CancelRequested || job.Status != shareddomain.ExportRunning, err
}

func (r *exportRepoSQL) filter(db *gorm.DB, f *domain.FilterExport) *gorm.DB {
	for col, v := range map[string]string{"type": f.Type, "status": f.Status, "requested_by": f.RequestedBy} {
		if v != "" {
			db = db.Where(col+" = ?", v)
		}
	}
	return db
}

func (r *exportRepoSQL) FetchAll(ctx context.Context, f *domain.FilterExport) (result []shareddomain.ExportJob, err error) {
	db := r.filter(r.db(ctx).Model(&shareddomain.ExportJob{}), f)
	err = gormx.ApplyPaging(db, &f.Filter, "created_at", "created_at", "updated_at").Find(&result).Error
	return
}

func (r *exportRepoSQL) Count(ctx context.Context, f *domain.FilterExport) int {
	var total int64
	r.filter(r.db(ctx).Model(&shareddomain.ExportJob{}), f).Count(&total)
	return int(total)
}

func (r *exportRepoSQL) FetchStale(ctx context.Context, queuedBefore, heartbeatBefore time.Time, limit int) (result []shareddomain.ExportJob, err error) {
	err = r.db(ctx).Where("(status = ? AND updated_at < ?) OR (status = ? AND COALESCE(heartbeat_at, updated_at) < ?)",
		shareddomain.ExportQueued, queuedBefore, shareddomain.ExportRunning, heartbeatBefore).
		Order("created_at").Limit(limit).Find(&result).Error
	return
}

func (r *exportRepoSQL) FetchExpired(ctx context.Context, now time.Time, limit int) (result []shareddomain.ExportJob, err error) {
	err = r.db(ctx).Where("status = ? AND expires_at < ?", shareddomain.ExportCompleted, now).
		Order("expires_at").Limit(limit).Find(&result).Error
	return
}
