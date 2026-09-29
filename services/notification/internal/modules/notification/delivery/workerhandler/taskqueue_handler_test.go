package workerhandler

import (
	"bytes"
	"context"
	"errors"
	"testing"

	mockusecase "monorepo/services/notification/pkg/mocks/modules/notification/usecase"
	mocksharedusecase "monorepo/services/notification/pkg/mocks/shared/usecase"

	"github.com/golangid/candi/candishared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestTaskQueueHandler_handleTaskNotificationDispatch(t *testing.T) {
	reqBody := []byte(`{"channel":"email","templateCode":"otp_verification","recipient":"a@b.com","renderedBody":"hi"}`)

	t.Run("Positive", func(t *testing.T) {
		notificationUsecase := &mockusecase.NotificationUsecase{}
		notificationUsecase.On("DispatchNotification", mock.Anything, mock.Anything).Return(nil)

		uc := &mocksharedusecase.Usecase{}
		uc.On("Notification").Return(notificationUsecase)

		h := TaskQueueHandler{uc: uc}
		ctx := candishared.NewEventContext(&bytes.Buffer{})
		ctx.SetContext(context.Background())
		ctx.WriteString(string(reqBody))

		err := h.handleTaskNotificationDispatch(ctx)
		assert.NoError(t, err)
	})

	t.Run("Negative bad payload", func(t *testing.T) {
		h := TaskQueueHandler{}
		ctx := candishared.NewEventContext(&bytes.Buffer{})
		ctx.SetContext(context.Background())
		ctx.WriteString("not-json")

		err := h.handleTaskNotificationDispatch(ctx)
		assert.Error(t, err)
	})

	t.Run("Negative dispatch error triggers retry", func(t *testing.T) {
		notificationUsecase := &mockusecase.NotificationUsecase{}
		notificationUsecase.On("DispatchNotification", mock.Anything, mock.Anything).Return(errors.New("smtp down"))

		uc := &mocksharedusecase.Usecase{}
		uc.On("Notification").Return(notificationUsecase)

		h := TaskQueueHandler{uc: uc}
		ctx := candishared.NewEventContext(&bytes.Buffer{})
		ctx.SetContext(context.Background())
		ctx.WriteString(string(reqBody))

		err := h.handleTaskNotificationDispatch(ctx)
		assert.Error(t, err)
		_, ok := err.(*candishared.ErrorRetrier)
		assert.True(t, ok)
	})
}
