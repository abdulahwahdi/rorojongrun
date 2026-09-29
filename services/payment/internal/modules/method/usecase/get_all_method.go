package usecase

import (
	"context"

	"monorepo/services/payment/internal/modules/method/domain"
	shareddomain "monorepo/services/payment/pkg/shared/domain"

	"github.com/golangid/candi/candishared"
	"github.com/golangid/candi/tracer"
)

func (uc *methodUsecaseImpl) GetAllMethods(ctx context.Context, filter *domain.FilterMethod) (data domain.ResponseMethodList, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "MethodUsecase:GetAllMethods")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	repo := uc.repoSQL.MethodRepo()
	rows, err := repo.FetchAll(ctx, filter)
	if err != nil {
		return data, err
	}
	data.Data = rows
	if data.Data == nil {
		data.Data = []shareddomain.Method{}
	}
	count := repo.Count(ctx, filter)
	data.Meta = candishared.NewMeta(filter.Page, filter.Limit, count)
	return data, nil
}
