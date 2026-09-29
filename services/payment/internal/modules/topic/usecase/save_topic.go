package usecase

import (
	"context"
	"errors"
	"regexp"
	"slices"

	"monorepo/globalshared/rest"
	"monorepo/services/payment/internal/modules/topic/domain"
	"monorepo/services/payment/pkg/helper"
	"monorepo/services/payment/pkg/shared"
	shareddomain "monorepo/services/payment/pkg/shared/domain"

	"github.com/golangid/candi/tracer"
	"gorm.io/gorm"
)

var topicNameRe = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,200}$`)

var publishEventTypes = []string{
	shareddomain.EventPaymentCreated, shareddomain.EventCheckoutStarted, shareddomain.EventPaymentCompleted, shareddomain.EventPaymentExpired,
	shareddomain.EventPaymentCancelled, shareddomain.EventNotificationMail,
}

func (uc *topicUsecaseImpl) validate(ctx context.Context, req *domain.RequestSaveTopic) error {
	if !topicNameRe.MatchString(req.Topic) {
		return rest.NewInvalid("topic may only contain letters, digits, '.', '_' and '-'")
	}
	switch req.Direction {
	case shareddomain.TopicConsume:
		if req.GatewayCode == "" || req.EventType != "" {
			return rest.NewInvalid("a consume topic needs gatewayCode and no eventType")
		}
		if _, err := uc.repoSQL.GatewayRepo().FindByCode(ctx, req.GatewayCode); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return rest.NewInvalid("unknown gateway " + req.GatewayCode)
			}
			return err
		}
	case shareddomain.TopicPublish:
		if req.GatewayCode != "" || !slices.Contains(publishEventTypes, req.EventType) {
			return rest.NewInvalid("a publish topic needs an eventType (one of " + joinComma(publishEventTypes) + ") and no gatewayCode")
		}
	default:
		return rest.NewInvalid("direction must be consume or publish")
	}
	return nil
}

func joinComma(s []string) (out string) {
	for i, v := range s {
		if i > 0 {
			out += ", "
		}
		out += v
	}
	return
}

func apply(t *shareddomain.Topic, req *domain.RequestSaveTopic) {
	t.Topic, t.Direction, t.IsEnabled = req.Topic, req.Direction, req.IsEnabled
	t.GatewayCode, t.EventType = helper.StrPtr(req.GatewayCode), helper.StrPtr(req.EventType)
}

func (uc *topicUsecaseImpl) CreateTopic(ctx context.Context, req *domain.RequestSaveTopic) (data shareddomain.Topic, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "TopicUsecase:CreateTopic")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	if err = uc.validate(ctx, req); err != nil {
		return data, err
	}
	apply(&data, req)
	if err = uc.repoSQL.TopicRepo().Save(ctx, &data); err != nil {
		return shareddomain.Topic{}, rest.MapDBError(err)
	}
	shared.NotifyTopicsChanged()
	return data, nil
}

func (uc *topicUsecaseImpl) UpdateTopic(ctx context.Context, id int, req *domain.RequestSaveTopic) (data shareddomain.Topic, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "TopicUsecase:UpdateTopic")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	if data, err = uc.GetTopic(ctx, id); err != nil {
		return data, err
	}
	if err = uc.validate(ctx, req); err != nil {
		return data, err
	}
	apply(&data, req)
	if err = uc.repoSQL.TopicRepo().Save(ctx, &data); err != nil {
		return shareddomain.Topic{}, rest.MapDBError(err)
	}
	shared.NotifyTopicsChanged()
	return data, nil
}

func (uc *topicUsecaseImpl) SetTopicStatus(ctx context.Context, id int, enabled bool) (data shareddomain.Topic, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "TopicUsecase:SetTopicStatus")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	if data, err = uc.GetTopic(ctx, id); err != nil {
		return data, err
	}
	data.IsEnabled = enabled
	if err = uc.repoSQL.TopicRepo().Save(ctx, &data); err != nil {
		return data, err
	}
	shared.NotifyTopicsChanged()
	return data, nil
}

func (uc *topicUsecaseImpl) DeleteTopic(ctx context.Context, id int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "TopicUsecase:DeleteTopic")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	if _, err = uc.GetTopic(ctx, id); err != nil {
		return err
	}
	if err = uc.repoSQL.TopicRepo().Delete(ctx, id); err != nil {
		return err
	}
	shared.NotifyTopicsChanged()
	return nil
}
