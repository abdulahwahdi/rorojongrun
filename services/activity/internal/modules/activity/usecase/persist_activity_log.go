package usecase

import (
	"context"

	"monorepo/services/activity/internal/modules/activity/domain"

	"github.com/golangid/candi/tracer"
)

// PersistActivityLog does the actual Mongo write. Called only from the task
// queue worker handler, never directly from REST.
func (uc *activityUsecaseImpl) PersistActivityLog(ctx context.Context, req *domain.RequestSaveActivity) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "ActivityUsecase:PersistActivityLog")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	data := req.Deserialize()
	return uc.repoMongo.ActivityRepo().Save(ctx, &data)
}
