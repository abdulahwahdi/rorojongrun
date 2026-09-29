package usecase

import (
	"context"

	"monorepo/services/activity/internal/modules/activity/domain"

	"github.com/golangid/candi/tracer"
)

func (uc *activityUsecaseImpl) GetActivityLogByID(ctx context.Context, id string) (result domain.ResponseActivity, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "ActivityUsecase:GetActivityLogByID")
	defer trace.Finish()

	repoFilter := domain.FilterActivity{ID: &id}
	data, err := uc.repoMongo.ActivityRepo().Find(ctx, &repoFilter)
	if err != nil {
		return result, err
	}

	result.Serialize(&data)
	return
}
