package migrations

import (
	"monorepo/services/notification/pkg/shared/domain"
)

// GetMigrateTables get migrate table list
func GetMigrateTables() []any {
	return []any{
		domain.NotificationTemplate{},
		domain.NotificationChannelConfig{},
		domain.NotificationLog{},
		domain.OTPRequest{},
	}
}
