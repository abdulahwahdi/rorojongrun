package usecase

import (
	"context"

	"monorepo/services/notification/internal/modules/notification/domain"

	"github.com/golangid/candi/candishared"
	"github.com/golangid/candi/tracer"
)

func (uc *notificationUsecaseImpl) GetAllTemplates(ctx context.Context, filter *domain.FilterTemplate) (result domain.ResponseTemplateList, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "NotificationUsecase:GetAllTemplates")
	defer trace.Finish()

	data, err := uc.repoSQL.NotificationRepo().FetchAllTemplates(ctx, filter)
	if err != nil {
		return result, err
	}
	count := uc.repoSQL.NotificationRepo().CountTemplates(ctx, filter)
	result.Meta = candishared.NewMeta(filter.Page, filter.Limit, count)

	result.Data = make([]domain.ResponseTemplate, len(data))
	for i, detail := range data {
		result.Data[i].Serialize(&detail)
	}
	return
}
