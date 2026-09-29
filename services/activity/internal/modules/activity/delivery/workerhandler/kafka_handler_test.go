package workerhandler

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"monorepo/services/activity/internal/modules/activity/domain"
	mockusecase "monorepo/services/activity/pkg/mocks/modules/activity/usecase"
	mocksharedusecase "monorepo/services/activity/pkg/mocks/shared/usecase"

	"github.com/golangid/candi/candishared"
	"github.com/golangid/candi/codebase/factory/types"
	"github.com/golangid/candi/validator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newEvent(body string) *candishared.EventContext {
	ctx := candishared.NewEventContext(&bytes.Buffer{})
	ctx.SetContext(context.Background())
	ctx.WriteString(body)
	return ctx
}

func TestKafkaHandler_handleActivityRequested(t *testing.T) {
	v := validator.NewValidator()
	v.JSONSchema.SchemaStorage = validator.NewFileSystemStorage(testSchemas, "jsonschema")
	body := `{"serviceName":"order","eventType":"order.confirmed","referenceId":"KOP-20260929-000001","actorId":"u-1","metadata":{"orderId":1}}`

	t.Run("persists the entry", func(t *testing.T) {
		activityUsecase := &mockusecase.ActivityUsecase{}
		activityUsecase.On("PersistActivityLog", mock.Anything, mock.MatchedBy(func(r *domain.RequestSaveActivity) bool {
			return r.ServiceName == "order" && r.ReferenceID == "KOP-20260929-000001" && r.ActorID == "u-1"
		})).Return(nil).Once()
		uc := &mocksharedusecase.Usecase{}
		uc.On("Activity").Return(activityUsecase)

		h := KafkaHandler{uc: uc, validator: v}
		assert.NoError(t, h.handleActivityRequested(newEvent(body)))
		activityUsecase.AssertExpectations(t)
	})

	t.Run("an invalid message is skipped, not retried", func(t *testing.T) {
		h := KafkaHandler{uc: &mocksharedusecase.Usecase{}, validator: v}
		assert.NoError(t, h.handleActivityRequested(newEvent(`{"serviceName":"order"}`)))
		assert.NoError(t, h.handleActivityRequested(newEvent(`not-json`)))
	})

	t.Run("a persistence error is retried then returned", func(t *testing.T) {
		activityUsecase := &mockusecase.ActivityUsecase{}
		activityUsecase.On("PersistActivityLog", mock.Anything, mock.Anything).Return(errors.New("mongo down")).Times(3)
		uc := &mocksharedusecase.Usecase{}
		uc.On("Activity").Return(activityUsecase)

		h := KafkaHandler{uc: uc, validator: v}
		assert.Error(t, h.handleActivityRequested(newEvent(body)))
		activityUsecase.AssertExpectations(t)
	})

	t.Run("mounts activity.requested", func(t *testing.T) {
		var group types.WorkerHandlerGroup
		(&KafkaHandler{}).MountHandlers(&group)
		assert.Equal(t, "activity.requested", group.Handlers[0].Pattern)
	})
}
