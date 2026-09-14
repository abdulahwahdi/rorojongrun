package usecase

import (
	"context"

	"monorepo/services/notification/internal/modules/notification/domain"
	shareddomain "monorepo/services/notification/pkg/shared/domain"

	"github.com/golangid/candi/tracer"
)

func (uc *notificationUsecaseImpl) CreateTemplate(ctx context.Context, req *domain.RequestUpsertTemplate) (res domain.ResponseTemplate, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "NotificationUsecase:CreateTemplate")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	data := shareddomain.NotificationTemplate{
		Code:         req.Code,
		Channel:      req.Channel,
		BodyTemplate: req.BodyTemplate,
		IsActive:     req.IsActive,
	}
	if req.SubjectTemplate != "" {
		data.SubjectTemplate = &req.SubjectTemplate
	}
	if req.Variables != "" {
		data.Variables = &req.Variables
	}

	if err = uc.repoSQL.NotificationRepo().SaveTemplate(ctx, &data); err != nil {
		return res, err
	}
	res.Serialize(&data)
	return
}
