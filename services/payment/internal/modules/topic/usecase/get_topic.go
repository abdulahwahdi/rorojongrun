package usecase

import (
	"context"
	"errors"

	"monorepo/globalshared/rest"
	"monorepo/services/payment/internal/modules/topic/domain"
	shareddomain "monorepo/services/payment/pkg/shared/domain"

	"github.com/golangid/candi/candishared"
	"github.com/golangid/candi/tracer"
	"gorm.io/gorm"
)

func (uc *topicUsecaseImpl) GetAllTopics(ctx context.Context, filter *domain.FilterTopic) (data domain.ResponseTopicList, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "TopicUsecase:GetAllTopics")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	repo := uc.repoSQL.TopicRepo()
	if data.Data, err = repo.FetchAll(ctx, filter); err != nil {
		return data, err
	}
	if data.Data == nil {
		data.Data = []shareddomain.Topic{}
	}
	data.Meta = candishared.NewMeta(filter.Page, filter.Limit, repo.Count(ctx, filter))
	return data, nil
}

func (uc *topicUsecaseImpl) GetTopic(ctx context.Context, id int) (data shareddomain.Topic, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "TopicUsecase:GetTopic")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	data, err = uc.repoSQL.TopicRepo().FindByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return data, rest.NewNotFound("topic not found")
	}
	return data, err
}
