package usecase

import (
	"context"

	"monorepo/services/activity/internal/modules/activity/domain"

	"github.com/golangid/candi/candishared"
	"github.com/golangid/candi/tracer"
)

func (uc *activityUsecaseImpl) GetAllActivityLogs(ctx context.Context, filter *domain.FilterActivity) (result domain.ResponseActivityList, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "ActivityUsecase:GetAllActivityLogs")
	defer trace.Finish()

	data, err := uc.repoMongo.ActivityRepo().FetchAll(ctx, filter)
	if err != nil {
		return result, err
	}
	count := uc.repoMongo.ActivityRepo().Count(ctx, filter)
	result.Meta = candishared.NewMeta(filter.Page, filter.Limit, count)

	result.Data = make([]domain.ResponseActivity, len(data))
	for i, detail := range data {
		result.Data[i].Serialize(&detail)
	}

	return
}
