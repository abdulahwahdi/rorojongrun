package usecase

import (
	"context"

	"monorepo/services/user/internal/modules/menu/domain"
	shareddomain "monorepo/services/user/pkg/shared/domain"
	"monorepo/services/user/pkg/shared/repository"
	"monorepo/services/user/pkg/shared/usecase/common"

	"github.com/golangid/candi/codebase/factory/dependency"
)

// MenuUsecase abstraction — the UI menu tree of a client app, each node optionally tied to a permission
type MenuUsecase interface {
	// GetAllMenu returns a flat page, or the whole nested tree when filter.Tree is set
	GetAllMenu(ctx context.Context, realm, clientID string, filter *domain.FilterMenu) (data domain.ResponseMenuList, err error)
	GetDetailMenu(ctx context.Context, realm, clientID string, id int) (data domain.ResponseMenu, err error)
	CreateMenu(ctx context.Context, realm, clientID string, data *domain.RequestMenu) (res domain.ResponseMenu, err error)
	UpdateMenu(ctx context.Context, realm, clientID string, id int, data *domain.RequestMenu) (err error)
	// DeleteMenu also deletes every descendant
	DeleteMenu(ctx context.Context, realm, clientID string, id int) (err error)

	// GetUserMenu returns the nested menu of a client restricted to what the granted permissions allow.
	// Used by the auth module for GET /me/permissions, no authorization is applied here.
	GetUserMenu(ctx context.Context, realmID, clientDBID int, granted []shareddomain.Permission) (data []domain.ResponseMenu, err error)
}

type menuUsecaseImpl struct {
	deps          dependency.Dependency
	sharedUsecase common.Usecase
	repoSQL       repository.RepoSQL
}

// NewMenuUsecase usecase impl constructor
func NewMenuUsecase(deps dependency.Dependency) (MenuUsecase, func(sharedUsecase common.Usecase)) {
	uc := &menuUsecaseImpl{
		deps:    deps,
		repoSQL: repository.GetSharedRepoSQL(),
	}
	return uc, func(sharedUsecase common.Usecase) {
		uc.sharedUsecase = sharedUsecase
	}
}
