package usecase

import (
	"context"

	"monorepo/services/payment/internal/modules/gateway/domain"
	"monorepo/services/payment/internal/modules/gateway/provider"

	"github.com/golangid/candi/tracer"
)

func (uc *gatewayUsecaseImpl) GetAllGateways(ctx context.Context) (data []domain.ResponseGateway, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "GatewayUsecase:GetAllGateways")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	rows, err := uc.repoSQL.GatewayRepo().FetchAll(ctx)
	if err != nil {
		return nil, err
	}
	data = make([]domain.ResponseGateway, 0, len(rows))
	for i := range rows {
		creds := map[string]string{}
		if cfg, cerr := provider.BuildConfig(rows[i], uc.encryptionSecret()); cerr == nil {
			creds = cfg.Credentials
		}
		data = append(data, uc.response(&rows[i], creds))
	}
	return data, nil
}
