package usecase

import (
	"context"

	"monorepo/services/notification/internal/modules/notification/domain"

	"github.com/golangid/candi/candishared"
	"github.com/golangid/candi/tracer"
)

func (uc *notificationUsecaseImpl) GetAllNotificationLogs(ctx context.Context, filter *domain.FilterNotificationLog) (result domain.ResponseNotificationLogList, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "NotificationUsecase:GetAllNotificationLogs")
	defer trace.Finish()

	data, err := uc.repoSQL.NotificationRepo().FetchAllLogs(ctx, filter)
	if err != nil {
		return result, err
	}
	count := uc.repoSQL.NotificationRepo().CountLogs(ctx, filter)
	result.Meta = candishared.NewMeta(filter.Page, filter.Limit, count)

	result.Data = make([]domain.ResponseNotificationLog, len(data))
	for i, detail := range data {
		result.Data[i].Serialize(&detail)
	}
	return
}
