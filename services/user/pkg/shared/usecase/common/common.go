package common

import (
	"context"

	// Only the menu module's domain package is imported here (not its usecase package): every
	// module's usecase imports this common package, so referencing another module's usecase
	// interface here would create an import cycle. See pkg/shared/usecase/usecase.go for the bridge.
	menudomain "monorepo/services/user/internal/modules/menu/domain"
	shareddomain "monorepo/services/user/pkg/shared/domain"
)

var commonUC Usecase

// Usecase common abstraction for bridging shared method inter usecase in module
type Usecase interface {
	// GetUserMenu is implemented by the menu usecase, used by auth for GET /me/permissions
	GetUserMenu(ctx context.Context, realmID, clientDBID int, granted []shareddomain.Permission) ([]menudomain.ResponseMenu, error)
}

// SetCommonUsecase constructor
func SetCommonUsecase(uc Usecase) Usecase {
	commonUC = uc
	return commonUC
}

// GetCommonUsecase get common usecase
func GetCommonUsecase() Usecase {
	return commonUC
}
