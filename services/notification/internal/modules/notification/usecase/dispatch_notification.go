package usecase

import (
	"context"
	"time"

	"monorepo/services/notification/internal/modules/notification/domain"
	shareddomain "monorepo/services/notification/pkg/shared/domain"

	"github.com/golangid/candi/tracer"
)

// DispatchNotification does the actual send via the channel's Sender.
// Called only from the task queue worker handler.
func (uc *notificationUsecaseImpl) DispatchNotification(ctx context.Context, payload *domain.DispatchPayload) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "NotificationUsecase:DispatchNotification")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	var subjectPtr, bodyPtr *string
	if payload.RenderedSubject != "" {
		subjectPtr = &payload.RenderedSubject
	}
	if payload.RenderedBody != "" {
		bodyPtr = &payload.RenderedBody
	}

	log := shareddomain.NotificationLog{
		Channel:         payload.Channel,
		TemplateCode:    payload.TemplateCode,
		Recipient:       payload.Recipient,
		RenderedSubject: subjectPtr,
		RenderedBody:    bodyPtr,
		Status:          domain.StatusPending,
	}
	if err = uc.repoSQL.NotificationRepo().SaveLog(ctx, &log); err != nil {
		return err
	}

	sender, ok := uc.senders[payload.Channel]
	if !ok {
		errMsg := "no sender registered for channel " + payload.Channel
		uc.repoSQL.NotificationRepo().UpdateLogStatus(ctx, log.ID, domain.StatusFailed, &errMsg, nil)
		return nil // non-retriable: no sender will ever exist for this channel
	}

	sendErr := sender.Send(ctx, payload.Recipient, payload.RenderedSubject, payload.RenderedBody)
	if sendErr != nil {
		errMsg := sendErr.Error()
		uc.repoSQL.NotificationRepo().UpdateLogStatus(ctx, log.ID, domain.StatusFailed, &errMsg, nil)
		return sendErr
	}

	now := time.Now()
	return uc.repoSQL.NotificationRepo().UpdateLogStatus(ctx, log.ID, domain.StatusSent, nil, &now)
}
