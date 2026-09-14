package repository

import (
	"context"
	"strings"
	"time"

	"monorepo/services/notification/internal/modules/notification/domain"
	shareddomain "monorepo/services/notification/pkg/shared/domain"

	"github.com/golangid/candi/tracer"

	"monorepo/globalshared"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type notificationRepoSQL struct {
	readDB, writeDB *gorm.DB
}

// NewNotificationRepoSQL sql repo constructor
func NewNotificationRepoSQL(readDB, writeDB *gorm.DB) NotificationRepository {
	return &notificationRepoSQL{readDB: readDB, writeDB: writeDB}
}

// --- Templates ---

func (r *notificationRepoSQL) FindTemplateByCodeChannel(ctx context.Context, code, channel string) (result shareddomain.NotificationTemplate, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "NotificationRepoSQL:FindTemplateByCodeChannel")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = globalshared.SetSpanToGorm(ctx, r.readDB).
		Where("code = ? AND channel = ?", code, channel).First(&result).Error
	return
}

func (r *notificationRepoSQL) FindTemplateByID(ctx context.Context, id int) (result shareddomain.NotificationTemplate, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "NotificationRepoSQL:FindTemplateByID")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = globalshared.SetSpanToGorm(ctx, r.readDB).Where("id = ?", id).First(&result).Error
	return
}

func (r *notificationRepoSQL) SaveTemplate(ctx context.Context, data *shareddomain.NotificationTemplate) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "NotificationRepoSQL:SaveTemplate")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	data.UpdatedAt = time.Now()
	if data.ID == 0 {
		data.CreatedAt = time.Now()
		err = globalshared.SetSpanToGorm(ctx, r.writeDB).Create(data).Error
	} else {
		err = globalshared.SetSpanToGorm(ctx, r.writeDB).Save(data).Error
	}
	return
}

func (r *notificationRepoSQL) FetchAllTemplates(ctx context.Context, filter *domain.FilterTemplate) (data []shareddomain.NotificationTemplate, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "NotificationRepoSQL:FetchAllTemplates")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	if filter.OrderBy == "" {
		filter.OrderBy = "updated_at"
	}
	db := r.setFilterTemplate(globalshared.SetSpanToGorm(ctx, r.readDB), filter).Order(clause.OrderByColumn{
		Column: clause.Column{Name: filter.OrderBy},
		Desc:   strings.ToUpper(filter.Sort) == "DESC",
	})
	if filter.Limit > 0 || !filter.ShowAll {
		db = db.Limit(filter.Limit).Offset(filter.CalculateOffset())
	}
	err = db.Find(&data).Error
	return
}

func (r *notificationRepoSQL) CountTemplates(ctx context.Context, filter *domain.FilterTemplate) (count int) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "NotificationRepoSQL:CountTemplates")
	defer trace.Finish()

	var total int64
	r.setFilterTemplate(globalshared.SetSpanToGorm(ctx, r.readDB), filter).Model(&shareddomain.NotificationTemplate{}).Count(&total)
	return int(total)
}

func (r *notificationRepoSQL) DeleteTemplate(ctx context.Context, id int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "NotificationRepoSQL:DeleteTemplate")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = globalshared.SetSpanToGorm(ctx, r.writeDB).Where("id = ?", id).Delete(&shareddomain.NotificationTemplate{}).Error
	return
}

func (r *notificationRepoSQL) setFilterTemplate(db *gorm.DB, filter *domain.FilterTemplate) *gorm.DB {
	if filter.Channel != "" {
		db = db.Where("channel = ?", filter.Channel)
	}
	if filter.Search != "" {
		db = db.Where("code ILIKE '%%' || ? || '%%'", filter.Search)
	}
	return db
}

// --- Channel configs ---

func (r *notificationRepoSQL) FindChannelConfig(ctx context.Context, channel string) (result shareddomain.NotificationChannelConfig, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "NotificationRepoSQL:FindChannelConfig")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = globalshared.SetSpanToGorm(ctx, r.readDB).Where("channel = ?", channel).First(&result).Error
	return
}

func (r *notificationRepoSQL) SaveChannelConfig(ctx context.Context, data *shareddomain.NotificationChannelConfig) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "NotificationRepoSQL:SaveChannelConfig")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	data.UpdatedAt = time.Now()
	if data.ID == 0 {
		data.CreatedAt = time.Now()
		err = globalshared.SetSpanToGorm(ctx, r.writeDB).Create(data).Error
	} else {
		err = globalshared.SetSpanToGorm(ctx, r.writeDB).Save(data).Error
	}
	return
}

// --- Logs ---

func (r *notificationRepoSQL) SaveLog(ctx context.Context, data *shareddomain.NotificationLog) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "NotificationRepoSQL:SaveLog")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	data.CreatedAt = time.Now()
	data.UpdatedAt = time.Now()
	err = globalshared.SetSpanToGorm(ctx, r.writeDB).Create(data).Error
	return
}

func (r *notificationRepoSQL) UpdateLogStatus(ctx context.Context, id int, status string, errorMessage *string, sentAt *time.Time) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "NotificationRepoSQL:UpdateLogStatus")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	updates := map[string]any{
		"status":        status,
		"error_message": errorMessage,
		"sent_at":       sentAt,
		"updated_at":    time.Now(),
	}
	err = globalshared.SetSpanToGorm(ctx, r.writeDB).Model(&shareddomain.NotificationLog{}).
		Where("id = ?", id).Updates(updates).Error
	return
}

func (r *notificationRepoSQL) FindLogByID(ctx context.Context, id int) (result shareddomain.NotificationLog, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "NotificationRepoSQL:FindLogByID")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = globalshared.SetSpanToGorm(ctx, r.readDB).Where("id = ?", id).First(&result).Error
	return
}

func (r *notificationRepoSQL) FetchAllLogs(ctx context.Context, filter *domain.FilterNotificationLog) (data []shareddomain.NotificationLog, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "NotificationRepoSQL:FetchAllLogs")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	if filter.OrderBy == "" {
		filter.OrderBy = "updated_at"
	}
	db := r.setFilterLog(globalshared.SetSpanToGorm(ctx, r.readDB), filter).Order(clause.OrderByColumn{
		Column: clause.Column{Name: filter.OrderBy},
		Desc:   strings.ToUpper(filter.Sort) == "DESC",
	})
	if filter.Limit > 0 || !filter.ShowAll {
		db = db.Limit(filter.Limit).Offset(filter.CalculateOffset())
	}
	err = db.Find(&data).Error
	return
}

func (r *notificationRepoSQL) CountLogs(ctx context.Context, filter *domain.FilterNotificationLog) (count int) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "NotificationRepoSQL:CountLogs")
	defer trace.Finish()

	var total int64
	r.setFilterLog(globalshared.SetSpanToGorm(ctx, r.readDB), filter).Model(&shareddomain.NotificationLog{}).Count(&total)
	return int(total)
}

func (r *notificationRepoSQL) setFilterLog(db *gorm.DB, filter *domain.FilterNotificationLog) *gorm.DB {
	if filter.ID != nil {
		db = db.Where("id = ?", *filter.ID)
	}
	if filter.Channel != "" {
		db = db.Where("channel = ?", filter.Channel)
	}
	if filter.TemplateCode != "" {
		db = db.Where("template_code = ?", filter.TemplateCode)
	}
	if filter.Recipient != "" {
		db = db.Where("recipient = ?", filter.Recipient)
	}
	if filter.Status != "" {
		db = db.Where("status = ?", filter.Status)
	}
	return db
}
