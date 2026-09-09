package workerhandler

import (
	"bytes"
	"context"
	"errors"
	"testing"

	mockusecase "monorepo/services/activity/pkg/mocks/modules/activity/usecase"
	mocksharedusecase "monorepo/services/activity/pkg/mocks/shared/usecase"

	"github.com/golangid/candi/candishared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestTaskQueueHandler_handleTaskActivity(t *testing.T) {
	reqBody := []byte(`{"serviceName":"order","eventType":"order.created","referenceId":"ord-123"}`)

	t.Run("Testcase #1: Positive", func(t *testing.T) {
		activityUsecase := &mockusecase.ActivityUsecase{}
		activityUsecase.On("PersistActivityLog", mock.Anything, mock.Anything).Return(nil)

		uc := &mocksharedusecase.Usecase{}
		uc.On("Activity").Return(activityUsecase)

		h := TaskQueueHandler{uc: uc}
		ctx := candishared.NewEventContext(&bytes.Buffer{})
		ctx.SetContext(context.Background())
		ctx.WriteString(string(reqBody))

		err := h.handleTaskActivity(ctx)
		assert.NoError(t, err)
	})

	t.Run("Testcase #2: Negative bad payload", func(t *testing.T) {
		h := TaskQueueHandler{}
		ctx := candishared.NewEventContext(&bytes.Buffer{})
		ctx.SetContext(context.Background())
		ctx.WriteString("not-json")

		err := h.handleTaskActivity(ctx)
		assert.Error(t, err)
	})

	t.Run("Testcase #3: Negative persist error triggers retry", func(t *testing.T) {
		activityUsecase := &mockusecase.ActivityUsecase{}
		activityUsecase.On("PersistActivityLog", mock.Anything, mock.Anything).Return(errors.New("db error"))

		uc := &mocksharedusecase.Usecase{}
		uc.On("Activity").Return(activityUsecase)

		h := TaskQueueHandler{uc: uc}
		ctx := candishared.NewEventContext(&bytes.Buffer{})
		ctx.SetContext(context.Background())
		ctx.WriteString(string(reqBody))

		err := h.handleTaskActivity(ctx)
		assert.Error(t, err)
		_, ok := err.(*candishared.ErrorRetrier)
		assert.True(t, ok)
	})
}
