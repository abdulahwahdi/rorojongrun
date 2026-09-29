package usecase

import (
	"context"

	"monorepo/services/payment/internal/modules/method/domain"
	"monorepo/services/payment/pkg/shared/repository"
	"monorepo/services/payment/pkg/shared/usecase/common"

	shareddomain "monorepo/services/payment/pkg/shared/domain"

	"github.com/golangid/candi/codebase/factory/dependency"
)

// MethodUsecase abstraction
type MethodUsecase interface {
	GetAllMethods(ctx context.Context, filter *domain.FilterMethod) (data domain.ResponseMethodList, err error)
	GetMethod(ctx context.Context, id int) (data shareddomain.Method, err error)
	CreateMethod(ctx context.Context, req *domain.RequestSaveMethod) (data shareddomain.Method, err error)
	UpdateMethod(ctx context.Context, id int, req *domain.RequestSaveMethod) (data shareddomain.Method, err error)
	SetMethodStatus(ctx context.Context, id int, enabled bool) (data shareddomain.Method, err error)
	DeleteMethod(ctx context.Context, id int) (err error)
}

type methodUsecaseImpl struct {
	deps          dependency.Dependency
	sharedUsecase common.Usecase
	repoSQL       repository.RepoSQL
}

// NewMethodUsecase usecase impl constructor
func NewMethodUsecase(deps dependency.Dependency) (MethodUsecase, func(sharedUsecase common.Usecase)) {
	uc := &methodUsecaseImpl{deps: deps, repoSQL: repository.GetSharedRepoSQL()}
	return uc, func(sharedUsecase common.Usecase) {
		uc.sharedUsecase = sharedUsecase
	}
}
