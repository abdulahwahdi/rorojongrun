package repository

import (
	"context"
	"time"

	"monorepo/services/notification/internal/modules/notification/domain"
	shareddomain "monorepo/services/notification/pkg/shared/domain"
)

// NotificationRepository abstract interface
type NotificationRepository interface {
	// Templates
	FindTemplateByCodeChannel(ctx context.Context, code, channel string) (shareddomain.NotificationTemplate, error)
	FindTemplateByID(ctx context.Context, id int) (shareddomain.NotificationTemplate, error)
	SaveTemplate(ctx context.Context, data *shareddomain.NotificationTemplate) error
	FetchAllTemplates(ctx context.Context, filter *domain.FilterTemplate) ([]shareddomain.NotificationTemplate, error)
	CountTemplates(ctx context.Context, filter *domain.FilterTemplate) int
	DeleteTemplate(ctx context.Context, id int) error

	// Channel configs
	FindChannelConfig(ctx context.Context, channel string) (shareddomain.NotificationChannelConfig, error)
	SaveChannelConfig(ctx context.Context, data *shareddomain.NotificationChannelConfig) error

	// Logs
	SaveLog(ctx context.Context, data *shareddomain.NotificationLog) error
	UpdateLogStatus(ctx context.Context, id int, status string, errorMessage *string, sentAt *time.Time) error
	FindLogByID(ctx context.Context, id int) (shareddomain.NotificationLog, error)
	FetchAllLogs(ctx context.Context, filter *domain.FilterNotificationLog) ([]shareddomain.NotificationLog, error)
	CountLogs(ctx context.Context, filter *domain.FilterNotificationLog) int
}
