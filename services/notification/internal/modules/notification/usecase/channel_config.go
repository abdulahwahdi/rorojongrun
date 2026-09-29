package usecase

import (
	"context"
	"encoding/json"

	"monorepo/services/notification/internal/modules/notification/domain"
	shareddomain "monorepo/services/notification/pkg/shared/domain"

	"github.com/golangid/candi/tracer"
)

func (uc *notificationUsecaseImpl) GetChannelConfig(ctx context.Context, channel string) (res domain.ResponseChannelConfig, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "NotificationUsecase:GetChannelConfig")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	data, err := uc.repoSQL.NotificationRepo().FindChannelConfig(ctx, channel)
	if err != nil {
		return res, err
	}
	res.Serialize(&data)
	return
}

func (uc *notificationUsecaseImpl) UpdateChannelConfig(ctx context.Context, channel string, req *domain.RequestUpsertChannelConfig) (res domain.ResponseChannelConfig, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "NotificationUsecase:UpdateChannelConfig")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	data, findErr := uc.repoSQL.NotificationRepo().FindChannelConfig(ctx, channel)
	if findErr != nil {
		data = shareddomain.NotificationChannelConfig{Channel: channel}
	}

	data.IsEnabled = req.IsEnabled
	if req.Settings != nil {
		settingsBytes, marshalErr := json.Marshal(req.Settings)
		if marshalErr != nil {
			return res, marshalErr
		}
		data.Settings = settingsBytes
	}

	if err = uc.repoSQL.NotificationRepo().SaveChannelConfig(ctx, &data); err != nil {
		return res, err
	}
	res.Serialize(&data)
	return
}
