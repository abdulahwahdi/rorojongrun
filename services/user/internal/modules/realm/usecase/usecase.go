package usecase

import (
	"context"

	"monorepo/globalshared/auth"
	"monorepo/services/user/internal/modules/realm/domain"
	"monorepo/services/user/pkg/shared/repository"
	"monorepo/services/user/pkg/shared/usecase/common"

	"github.com/golangid/candi/codebase/factory/dependency"
)

// RealmUsecase abstraction
type RealmUsecase interface {
	// realm CRUD — list / create / delete are master-realm only
	GetAllRealm(ctx context.Context, filter *domain.FilterRealm) (data domain.ResponseRealmList, err error)
	GetDetailRealm(ctx context.Context, name string) (data domain.ResponseRealm, err error)
	CreateRealm(ctx context.Context, data *domain.RequestRealm) (res domain.ResponseRealm, err error)
	UpdateRealm(ctx context.Context, name string, data *domain.RequestRealm) (err error)
	DeleteRealm(ctx context.Context, name string) (err error)

	// signing keys of a realm
	GetAllRealmKey(ctx context.Context, realm string, filter *domain.FilterRealmKey) (data domain.ResponseRealmKeyList, err error)
	GetDetailRealmKey(ctx context.Context, realm string, id int) (data domain.ResponseRealmKey, err error)
	RotateRealmKey(ctx context.Context, realm string) (res domain.ResponseRealmKey, err error)
	UpdateRealmKey(ctx context.Context, realm string, id int, data *domain.RequestRealmKey) (err error)
	DeleteRealmKey(ctx context.Context, realm string, id int) (err error)

	// public discovery documents
	GetJWKS(ctx context.Context, realm string) (data auth.JWKS, err error)
	GetOpenIDConfiguration(ctx context.Context, realm string) (data domain.ResponseOpenIDConfiguration, err error)
}

type realmUsecaseImpl struct {
	deps          dependency.Dependency
	sharedUsecase common.Usecase
	repoSQL       repository.RepoSQL
}

// NewRealmUsecase usecase impl constructor
func NewRealmUsecase(deps dependency.Dependency) (RealmUsecase, func(sharedUsecase common.Usecase)) {
	uc := &realmUsecaseImpl{
		deps:    deps,
		repoSQL: repository.GetSharedRepoSQL(),
	}
	return uc, func(sharedUsecase common.Usecase) {
		uc.sharedUsecase = sharedUsecase
	}
}
