package usecase

import (
	"context"
	"errors"

	"monorepo/globalshared/rest"
	shareddomain "monorepo/services/payment/pkg/shared/domain"

	"github.com/golangid/candi/tracer"
	"gorm.io/gorm"
)

func (uc *methodUsecaseImpl) GetMethod(ctx context.Context, id int) (data shareddomain.Method, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "MethodUsecase:GetMethod")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	data, err = uc.repoSQL.MethodRepo().FindByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return data, rest.NewNotFound("payment method not found")
	}
	return data, err
}
