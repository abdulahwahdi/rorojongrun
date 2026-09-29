package usecase

import (
	"context"

	"monorepo/services/user/internal/modules/client/domain"
	"monorepo/services/user/pkg/shared/repository"
	"monorepo/services/user/pkg/shared/usecase/common"

	"github.com/golangid/candi/codebase/factory/dependency"
)

// ClientUsecase abstraction — applications (public) and backend services (confidential) of a realm
type ClientUsecase interface {
	GetAllClient(ctx context.Context, realm string, filter *domain.FilterClient) (data domain.ResponseClientList, err error)
	GetDetailClient(ctx context.Context, realm string, id int) (data domain.ResponseClient, err error)
	// CreateClient of type confidential returns the generated secret once
	CreateClient(ctx context.Context, realm string, data *domain.RequestCreateClient) (res domain.ResponseClient, err error)
	UpdateClient(ctx context.Context, realm string, id int, data *domain.RequestUpdateClient) (err error)
	DeleteClient(ctx context.Context, realm string, id int) (err error)
	// RotateClientSecret invalidates the old secret and returns the new one once
	RotateClientSecret(ctx context.Context, realm string, id int) (res domain.ResponseClient, err error)
}

type clientUsecaseImpl struct {
	deps          dependency.Dependency
	sharedUsecase common.Usecase
	repoSQL       repository.RepoSQL
}

// NewClientUsecase usecase impl constructor
func NewClientUsecase(deps dependency.Dependency) (ClientUsecase, func(sharedUsecase common.Usecase)) {
	uc := &clientUsecaseImpl{
		deps:    deps,
		repoSQL: repository.GetSharedRepoSQL(),
	}
	return uc, func(sharedUsecase common.Usecase) {
		uc.sharedUsecase = sharedUsecase
	}
}
