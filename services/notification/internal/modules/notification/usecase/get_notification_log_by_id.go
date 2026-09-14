package usecase

import (
	"context"

	"monorepo/services/notification/internal/modules/notification/domain"

	"github.com/golangid/candi/tracer"
)

func (uc *notificationUsecaseImpl) GetNotificationLogByID(ctx context.Context, id int) (result domain.ResponseNotificationLog, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "NotificationUsecase:GetNotificationLogByID")
	defer trace.Finish()

	data, err := uc.repoSQL.NotificationRepo().FindLogByID(ctx, id)
	if err != nil {
		return result, err
	}
	result.Serialize(&data)
	return
}
