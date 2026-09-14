package usecase

import (
	"context"

	"monorepo/services/notification/internal/modules/notification/domain"

	"github.com/golangid/candi/tracer"
)

func (uc *notificationUsecaseImpl) UpdateTemplate(ctx context.Context, id int, req *domain.RequestUpsertTemplate) (res domain.ResponseTemplate, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "NotificationUsecase:UpdateTemplate")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	data, err := uc.repoSQL.NotificationRepo().FindTemplateByID(ctx, id)
	if err != nil {
		return res, err
	}

	data.Code = req.Code
	data.Channel = req.Channel
	data.BodyTemplate = req.BodyTemplate
	data.IsActive = req.IsActive
	if req.SubjectTemplate != "" {
		data.SubjectTemplate = &req.SubjectTemplate
	} else {
		data.SubjectTemplate = nil
	}
	if req.Variables != "" {
		data.Variables = &req.Variables
	} else {
		data.Variables = nil
	}

	if err = uc.repoSQL.NotificationRepo().SaveTemplate(ctx, &data); err != nil {
		return res, err
	}
	res.Serialize(&data)
	return
}
