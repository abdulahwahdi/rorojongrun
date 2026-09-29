package usecase

import (
	"context"

	"monorepo/services/payment/internal/modules/gateway/domain"

	"github.com/golangid/candi/tracer"
)

func (uc *gatewayUsecaseImpl) GetGateway(ctx context.Context, code string) (data domain.ResponseGateway, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "GatewayUsecase:GetGateway")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	row, creds, err := uc.load(ctx, code)
	if err != nil {
		return data, err
	}
	return uc.response(&row, creds), nil
}
