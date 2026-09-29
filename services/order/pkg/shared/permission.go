package shared

import (
	"context"

	"monorepo/globalshared/auth"

	"github.com/golangid/candi/codebase/interfaces"
)

var permissionChecker interfaces.ACLPermissionChecker

// SetPermissionChecker keeps the ACL checker of the middleware for checks a handler makes itself
func SetPermissionChecker(c interfaces.ACLPermissionChecker) { permissionChecker = c }

// HasPermission tells whether the caller of ctx holds a permission code of this service, on top
// of the one its route requires (e.g. manageExports to see everybody's exports)
func HasPermission(ctx context.Context, code string) bool {
	subject := auth.SubjectFromContext(ctx)
	if permissionChecker == nil || subject == "" {
		return false
	}
	_, err := permissionChecker.CheckPermission(ctx, subject, code)
	return err == nil
}
