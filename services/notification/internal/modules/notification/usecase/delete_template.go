package usecase

import (
	"context"

	"github.com/golangid/candi/tracer"
)

func (uc *notificationUsecaseImpl) DeleteTemplate(ctx context.Context, id int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "NotificationUsecase:DeleteTemplate")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	return uc.repoSQL.NotificationRepo().DeleteTemplate(ctx, id)
}
