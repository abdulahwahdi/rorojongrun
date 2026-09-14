package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	texttemplate "text/template"

	"monorepo/services/notification/internal/modules/notification/channel"
	"monorepo/services/notification/internal/modules/notification/domain"

	taskqueueworker "github.com/golangid/candi/codebase/app/task_queue_worker"
	"github.com/golangid/candi/tracer"
)

// SendNotification looks up the template + channel config, renders the
// message, and enqueues an async dispatch job — it never touches a Sender
// or writes a log row itself, that all happens in DispatchNotification.
func (uc *notificationUsecaseImpl) SendNotification(ctx context.Context, req *domain.RequestSendNotification) (jobID string, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "NotificationUsecase:SendNotification")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	tmpl, err := uc.repoSQL.NotificationRepo().FindTemplateByCodeChannel(ctx, req.TemplateCode, req.Channel)
	if err != nil {
		return "", fmt.Errorf("template not found for code=%s channel=%s: %w", req.TemplateCode, req.Channel, err)
	}
	if !tmpl.IsActive {
		return "", fmt.Errorf("template %s is not active", req.TemplateCode)
	}

	cfg, err := uc.repoSQL.NotificationRepo().FindChannelConfig(ctx, req.Channel)
	if err != nil || !cfg.IsEnabled {
		return "", channel.ErrChannelDisabled
	}

	var renderedSubject string
	if tmpl.SubjectTemplate != nil {
		subjTmpl, err := texttemplate.New("subject").Parse(*tmpl.SubjectTemplate)
		if err != nil {
			return "", fmt.Errorf("parse subject template: %w", err)
		}
		var buf bytes.Buffer
		if err := subjTmpl.Execute(&buf, req.Variables); err != nil {
			return "", fmt.Errorf("render subject template: %w", err)
		}
		renderedSubject = buf.String()
	}

	bodyTmpl, err := template.New("body").Parse(tmpl.BodyTemplate)
	if err != nil {
		return "", fmt.Errorf("parse body template: %w", err)
	}
	var bodyBuf bytes.Buffer
	if err := bodyTmpl.Execute(&bodyBuf, req.Variables); err != nil {
		return "", fmt.Errorf("render body template: %w", err)
	}

	args, err := json.Marshal(domain.DispatchPayload{
		Channel:         req.Channel,
		TemplateCode:    req.TemplateCode,
		Recipient:       req.Recipient,
		RenderedSubject: renderedSubject,
		RenderedBody:    bodyBuf.String(),
	})
	if err != nil {
		return "", err
	}

	jobID, err = uc.jobEnqueuer(ctx, &taskqueueworker.AddJobRequest{
		TaskName: domain.TaskNameNotificationDispatch,
		MaxRetry: 5,
		Args:     args,
	})
	return
}
