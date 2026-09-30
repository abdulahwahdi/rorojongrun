package usecase

import (
	"context"
	"errors"

	"monorepo/globalshared/rest"
	"monorepo/services/payment/internal/modules/gateway/domain"
	"monorepo/services/payment/internal/modules/gateway/provider"
	"monorepo/services/payment/pkg/shared"
	shareddomain "monorepo/services/payment/pkg/shared/domain"

	"gorm.io/gorm"
)

func gatewayEncryptionSecret() string { return shared.GetEnv().GatewayEncryptionSecret }

// load fetches a gateway row and its decrypted credentials
func (uc *gatewayUsecaseImpl) load(ctx context.Context, code string) (row shareddomain.Gateway, creds map[string]string, err error) {
	row, err = uc.repoSQL.GatewayRepo().FindByCode(ctx, code)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return row, nil, rest.NewNotFound("gateway not found")
	}
	if err != nil {
		return row, nil, err
	}
	cfg, err := provider.BuildConfig(row, uc.encryptionSecret())
	if err != nil {
		// keep the gateway visible/editable even when its credentials are unreadable
		return row, map[string]string{}, nil
	}
	return row, cfg.Credentials, nil
}

func (uc *gatewayUsecaseImpl) response(row *shareddomain.Gateway, creds map[string]string) domain.ResponseGateway {
	var res domain.ResponseGateway
	res.Serialize(row, creds)
	return res
}
