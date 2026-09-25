package usecase

import (
	"context"

	"monorepo/services/user/internal/modules/user/domain"
	"monorepo/services/user/pkg/shared/repository"
	"monorepo/services/user/pkg/shared/usecase/common"

	"github.com/golangid/candi/codebase/factory/dependency"
)

// UserUsecase abstraction — users of a realm and their role assignment
type UserUsecase interface {
	GetAllUser(ctx context.Context, realm string, filter *domain.FilterUser) (data domain.ResponseUserList, err error)
	GetDetailUser(ctx context.Context, realm string, id int) (data domain.ResponseUser, err error)
	CreateUser(ctx context.Context, realm string, data *domain.RequestCreateUser) (res domain.ResponseUser, err error)
	UpdateUser(ctx context.Context, realm string, id int, data *domain.RequestUpdateUser) (err error)
	DeleteUser(ctx context.Context, realm string, id int) (err error)
	SetPassword(ctx context.Context, realm string, id int, password string) (err error)
	// UnlockUser clears the failed-login lockout
	UnlockUser(ctx context.Context, realm string, id int) (err error)

	GetUserRoles(ctx context.Context, realm string, userID int) (data []domain.ResponseRoleRef, err error)
	AddUserRole(ctx context.Context, realm string, userID, roleID int) (err error)
	ReplaceUserRoles(ctx context.Context, realm string, userID int, roleIDs []int) (err error)
	RemoveUserRole(ctx context.Context, realm string, userID, roleID int) (err error)
}

type userUsecaseImpl struct {
	deps          dependency.Dependency
	sharedUsecase common.Usecase
	repoSQL       repository.RepoSQL
}

// NewUserUsecase usecase impl constructor
func NewUserUsecase(deps dependency.Dependency) (UserUsecase, func(sharedUsecase common.Usecase)) {
	uc := &userUsecaseImpl{
		deps:    deps,
		repoSQL: repository.GetSharedRepoSQL(),
	}
	return uc, func(sharedUsecase common.Usecase) {
		uc.sharedUsecase = sharedUsecase
	}
}
