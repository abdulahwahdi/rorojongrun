package usecase

import (
	"context"

	"monorepo/services/user/internal/modules/rbac/domain"
	"monorepo/services/user/pkg/shared/repository"
	"monorepo/services/user/pkg/shared/usecase/common"

	"github.com/golangid/candi/codebase/factory/dependency"
)

// RbacUsecase abstraction — roles, permissions and the role<->permission assignment of a realm
type RbacUsecase interface {
	GetAllRole(ctx context.Context, realm string, filter *domain.FilterRole) (data domain.ResponseRoleList, err error)
	GetDetailRole(ctx context.Context, realm string, id int) (data domain.ResponseRole, err error)
	CreateRole(ctx context.Context, realm string, data *domain.RequestRole) (res domain.ResponseRole, err error)
	UpdateRole(ctx context.Context, realm string, id int, data *domain.RequestRole) (err error)
	// DeleteRole is refused while users hold the role, unless force is set
	DeleteRole(ctx context.Context, realm string, id int, force bool) (err error)

	GetRolePermissions(ctx context.Context, realm string, roleID int) (data []domain.ResponsePermission, err error)
	AddRolePermission(ctx context.Context, realm string, roleID, permissionID int) (err error)
	ReplaceRolePermissions(ctx context.Context, realm string, roleID int, permissionIDs []int) (err error)
	RemoveRolePermission(ctx context.Context, realm string, roleID, permissionID int) (err error)

	GetAllPermission(ctx context.Context, realm string, filter *domain.FilterPermission) (data domain.ResponsePermissionList, err error)
	GetDetailPermission(ctx context.Context, realm string, id int) (data domain.ResponsePermission, err error)
	CreatePermission(ctx context.Context, realm string, data *domain.RequestPermission) (res domain.ResponsePermission, err error)
	UpdatePermission(ctx context.Context, realm string, id int, data *domain.RequestPermission) (err error)
	DeletePermission(ctx context.Context, realm string, id int) (err error)
	// UpsertPermissions creates or updates permissions by (service, code); used to register a service's codes
	UpsertPermissions(ctx context.Context, realm string, data []domain.RequestPermission) (res []domain.ResponsePermission, err error)
}

type rbacUsecaseImpl struct {
	deps          dependency.Dependency
	sharedUsecase common.Usecase
	repoSQL       repository.RepoSQL
}

// NewRbacUsecase usecase impl constructor
func NewRbacUsecase(deps dependency.Dependency) (RbacUsecase, func(sharedUsecase common.Usecase)) {
	uc := &rbacUsecaseImpl{
		deps:    deps,
		repoSQL: repository.GetSharedRepoSQL(),
	}
	return uc, func(sharedUsecase common.Usecase) {
		uc.sharedUsecase = sharedUsecase
	}
}
