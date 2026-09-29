package usecase

import (
	"context"

	"monorepo/services/payment/internal/modules/gateway/domain"
	"monorepo/services/payment/pkg/shared/repository"
	"monorepo/services/payment/pkg/shared/usecase/common"

	"github.com/golangid/candi/codebase/factory/dependency"
)

// GatewayUsecase abstraction
type GatewayUsecase interface {
	GetAllGateways(ctx context.Context) (data []domain.ResponseGateway, err error)
	GetGateway(ctx context.Context, code string) (data domain.ResponseGateway, err error)
	UpdateGateway(ctx context.Context, code string, req *domain.RequestUpdateGateway) (data domain.ResponseGateway, err error)
	SetGatewayStatus(ctx context.Context, code string, enabled bool) (data domain.ResponseGateway, err error)
}

type gatewayUsecaseImpl struct {
	deps          dependency.Dependency
	sharedUsecase common.Usecase
	repoSQL       repository.RepoSQL

	// encryptionSecret returns the key credentials are encrypted with (env, swappable in tests)
	encryptionSecret func() string
}

// NewGatewayUsecase usecase impl constructor
func NewGatewayUsecase(deps dependency.Dependency) (GatewayUsecase, func(sharedUsecase common.Usecase)) {
	uc := &gatewayUsecaseImpl{
		deps:             deps,
		repoSQL:          repository.GetSharedRepoSQL(),
		encryptionSecret: gatewayEncryptionSecret,
	}
	return uc, func(sharedUsecase common.Usecase) {
		uc.sharedUsecase = sharedUsecase
	}
}
