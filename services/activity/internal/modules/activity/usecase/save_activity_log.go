package usecase

import (
	"context"
	"encoding/json"

	"monorepo/services/activity/internal/modules/activity/domain"

	taskqueueworker "github.com/golangid/candi/codebase/app/task_queue_worker"
	"github.com/golangid/candi/tracer"
)

// SaveActivityLog marshals the request and enqueues it as a task queue job,
// returning immediately without waiting for the actual Mongo write.
func (uc *activityUsecaseImpl) SaveActivityLog(ctx context.Context, req *domain.RequestSaveActivity) (jobID string, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "ActivityUsecase:SaveActivityLog")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	args, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	jobID, err = uc.jobEnqueuer(ctx, &taskqueueworker.AddJobRequest{
		TaskName: domain.TaskNameActivity,
		MaxRetry: 3,
		Args:     args,
	})
	return
}
