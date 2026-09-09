package usecase

import (
	"context"
	"errors"
	"testing"

	"monorepo/services/activity/internal/modules/activity/domain"

	taskqueueworker "github.com/golangid/candi/codebase/app/task_queue_worker"

	"github.com/stretchr/testify/assert"
)

func Test_activityUsecaseImpl_SaveActivityLog(t *testing.T) {
	t.Run("Testcase #1: Positive", func(t *testing.T) {
		uc := activityUsecaseImpl{
			jobEnqueuer: func(ctx context.Context, req *taskqueueworker.AddJobRequest) (string, error) {
				assert.Equal(t, domain.TaskNameActivity, req.TaskName)
				return "job-id", nil
			},
		}

		jobID, err := uc.SaveActivityLog(context.Background(), &domain.RequestSaveActivity{
			ServiceName: "order", EventType: "order.created", ReferenceID: "ord-123",
		})
		assert.NoError(t, err)
		assert.Equal(t, "job-id", jobID)
	})

	t.Run("Testcase #2: Negative enqueue error", func(t *testing.T) {
		uc := activityUsecaseImpl{
			jobEnqueuer: func(ctx context.Context, req *taskqueueworker.AddJobRequest) (string, error) {
				return "", errors.New("enqueue failed")
			},
		}

		_, err := uc.SaveActivityLog(context.Background(), &domain.RequestSaveActivity{
			ServiceName: "order", EventType: "order.created", ReferenceID: "ord-123",
		})
		assert.Error(t, err)
	})
}
