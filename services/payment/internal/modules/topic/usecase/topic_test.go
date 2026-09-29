package usecase

import (
	"context"
	"testing"

	"monorepo/services/payment/internal/modules/topic/domain"
	"monorepo/services/payment/pkg/helper"
	mockgatewayrepo "monorepo/services/payment/pkg/mocks/modules/gateway/repository"
	mocktopicrepo "monorepo/services/payment/pkg/mocks/modules/topic/repository"
	mocksharedrepo "monorepo/services/payment/pkg/mocks/shared/repository"
	"monorepo/services/payment/pkg/shared"
	shareddomain "monorepo/services/payment/pkg/shared/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setup() (*topicUsecaseImpl, *mocktopicrepo.TopicRepository, *mockgatewayrepo.GatewayRepository) {
	topics, gateways := &mocktopicrepo.TopicRepository{}, &mockgatewayrepo.GatewayRepository{}
	repoSQL := &mocksharedrepo.RepoSQL{}
	repoSQL.On("TopicRepo").Return(topics)
	repoSQL.On("GatewayRepo").Return(gateways)
	return &topicUsecaseImpl{repoSQL: repoSQL}, topics, gateways
}

func drainSignal() {
	select {
	case <-shared.TopicsChanged():
	default:
	}
}

func signalled() bool {
	select {
	case <-shared.TopicsChanged():
		return true
	default:
		return false
	}
}

func Test_CreateTopic(t *testing.T) {
	ctx := context.Background()

	t.Run("consume topic wakes the callback consumer", func(t *testing.T) {
		drainSignal()
		uc, topics, gateways := setup()
		gateways.On("FindByCode", mock.Anything, "midtrans").Return(shareddomain.Gateway{Code: "midtrans"}, nil)
		var saved shareddomain.Topic
		topics.On("Save", mock.Anything, mock.Anything).Run(func(a mock.Arguments) { saved = *a.Get(1).(*shareddomain.Topic) }).Return(nil)

		_, err := uc.CreateTopic(ctx, &domain.RequestSaveTopic{Topic: "midtrans.callbacks.v2", Direction: "consume", GatewayCode: "midtrans", IsEnabled: true})
		require.NoError(t, err)
		assert.Equal(t, "midtrans", *saved.GatewayCode)
		assert.Nil(t, saved.EventType)
		assert.True(t, signalled(), "a change is applied without waiting for the reload interval")
	})

	t.Run("publish topic", func(t *testing.T) {
		uc, topics, _ := setup()
		var saved shareddomain.Topic
		topics.On("Save", mock.Anything, mock.Anything).Run(func(a mock.Arguments) { saved = *a.Get(1).(*shareddomain.Topic) }).Return(nil)
		_, err := uc.CreateTopic(ctx, &domain.RequestSaveTopic{Topic: "audit.checkout", Direction: "publish", EventType: "payment.checkout_started", IsEnabled: true})
		require.NoError(t, err)
		assert.Equal(t, "payment.checkout_started", *saved.EventType)
	})

	invalid := map[string]domain.RequestSaveTopic{
		"spaces in name":          {Topic: "bad topic", Direction: "publish", EventType: "payment.created"},
		"empty name":              {Topic: "", Direction: "publish", EventType: "payment.created"},
		"consume without gateway": {Topic: "t", Direction: "consume"},
		"consume with event type": {Topic: "t", Direction: "consume", GatewayCode: "midtrans", EventType: "payment.created"},
		"publish without event":   {Topic: "t", Direction: "publish"},
		"publish unknown event":   {Topic: "t", Direction: "publish", EventType: "payment.exploded"},
		"publish with gateway":    {Topic: "t", Direction: "publish", EventType: "payment.created", GatewayCode: "midtrans"},
		"unknown direction":       {Topic: "t", Direction: "sideways"},
	}
	for name, req := range invalid {
		t.Run("invalid: "+name, func(t *testing.T) {
			drainSignal()
			uc, topics, gateways := setup()
			gateways.On("FindByCode", mock.Anything, mock.Anything).Return(shareddomain.Gateway{Code: "midtrans"}, nil).Maybe()
			r := req
			_, err := uc.CreateTopic(ctx, &r)
			assert.Equal(t, 400, helper.HTTPStatus(err))
			topics.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
			assert.False(t, signalled())
		})
	}

	t.Run("invalid: consume topic of an unknown gateway", func(t *testing.T) {
		uc, _, gateways := setup()
		gateways.On("FindByCode", mock.Anything, "ghost").Return(shareddomain.Gateway{}, gorm.ErrRecordNotFound)
		_, err := uc.CreateTopic(ctx, &domain.RequestSaveTopic{Topic: "t", Direction: "consume", GatewayCode: "ghost"})
		assert.Equal(t, 400, helper.HTTPStatus(err))
	})
}

func Test_TopicStatusAndDelete(t *testing.T) {
	ctx := context.Background()
	uc, topics, _ := setup()
	topics.On("FindByID", mock.Anything, 1).Return(shareddomain.Topic{ID: 1, Topic: "t", IsEnabled: true}, nil)
	topics.On("FindByID", mock.Anything, 404).Return(shareddomain.Topic{}, gorm.ErrRecordNotFound)
	var saved shareddomain.Topic
	topics.On("Save", mock.Anything, mock.Anything).Run(func(a mock.Arguments) { saved = *a.Get(1).(*shareddomain.Topic) }).Return(nil)
	topics.On("Delete", mock.Anything, 1).Return(nil)

	drainSignal()
	_, err := uc.SetTopicStatus(ctx, 1, false)
	require.NoError(t, err)
	assert.False(t, saved.IsEnabled)
	assert.True(t, signalled(), "disabling a topic stops consuming it without a restart")

	assert.Equal(t, 404, helper.HTTPStatus(func() error { _, err := uc.SetTopicStatus(ctx, 404, true); return err }()))

	drainSignal()
	require.NoError(t, uc.DeleteTopic(ctx, 1))
	assert.True(t, signalled())
}
