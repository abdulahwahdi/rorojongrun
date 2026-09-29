package repository

import (
	"context"

	"monorepo/services/payment/internal/modules/topic/domain"
	shareddomain "monorepo/services/payment/pkg/shared/domain"
)

// TopicRepository abstract interface
type TopicRepository interface {
	FetchAll(ctx context.Context, filter *domain.FilterTopic) ([]shareddomain.Topic, error)
	Count(ctx context.Context, filter *domain.FilterTopic) int
	FindByID(ctx context.Context, id int) (shareddomain.Topic, error)
	Save(ctx context.Context, data *shareddomain.Topic) error
	Delete(ctx context.Context, id int) error

	// FetchEnabledConsume returns enabled consume topics whose gateway is enabled too:
	// this is what the callback consumer subscribes to
	FetchEnabledConsume(ctx context.Context) ([]shareddomain.Topic, error)
	// FetchEnabledPublish returns the enabled publish topics an outbound event type fans out to
	FetchEnabledPublish(ctx context.Context, eventType string) ([]shareddomain.Topic, error)
	// FindConsumeByTopic finds a consume topic by name regardless of enabled state
	FindConsumeByTopic(ctx context.Context, topic string) (shareddomain.Topic, error)
}
