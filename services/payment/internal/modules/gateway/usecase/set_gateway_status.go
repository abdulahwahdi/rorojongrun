package usecase

import (
	"context"

	"monorepo/services/payment/internal/modules/gateway/domain"
	"monorepo/services/payment/pkg/shared"

	"github.com/golangid/candi/tracer"
)

// SetGatewayStatus enables/disables a gateway. Enabling requires its credentials. Disabling hides
// its methods from checkout and stops consuming its callback topics (already issued
// charges keep being settled by callbacks that arrive before the consumer reloads).
func (uc *gatewayUsecaseImpl) SetGatewayStatus(ctx context.Context, code string, enabled bool) (data domain.ResponseGateway, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "GatewayUsecase:SetGatewayStatus")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	row, creds, err := uc.load(ctx, code)
	if err != nil {
		return data, err
	}
	if enabled {
		if err = requireCredentials(code, creds); err != nil {
			return data, err
		}
	}
	row.IsEnabled = enabled
	if err = uc.repoSQL.GatewayRepo().Save(ctx, &row); err != nil {
		return data, err
	}
	shared.NotifyTopicsChanged()
	return uc.response(&row, creds), nil
}
