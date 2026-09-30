package resthandler

import (
	"net/http"

	"monorepo/globalshared/rest"
	"monorepo/services/user/internal/modules/rbac/domain"
	"monorepo/services/user/pkg/shared/usecase"

	"github.com/golangid/candi/candihelper"
	restserver "github.com/golangid/candi/codebase/app/rest_server"
	"github.com/golangid/candi/codebase/factory/dependency"
	"github.com/golangid/candi/codebase/interfaces"
	"github.com/golangid/candi/tracer"
	"github.com/golangid/candi/wrapper"
)

// RestHandler handler
type RestHandler struct {
	mw        interfaces.Middleware
	uc        usecase.Usecase
	validator interfaces.Validator
}

// NewRestHandler create new rest handler
func NewRestHandler(uc usecase.Usecase, deps dependency.Dependency) *RestHandler {
	return &RestHandler{
		uc: uc, mw: deps.GetMiddleware(), validator: deps.GetValidator(),
	}
}

// Mount handler with root "/" (flat routes, see realm resthandler for why)
func (h *RestHandler) Mount(root interfaces.RESTRouter) {
	base := candihelper.V1 + "/realms/:realm"

	root.GET(base+"/roles", h.getAllRole, rest.Secure(h.mw, "getAllRole")...)
	root.POST(base+"/roles", h.createRole, rest.Secure(h.mw, "createRole")...)
	root.GET(base+"/roles/:id", h.getDetailRole, rest.Secure(h.mw, "getDetailRole")...)
	root.PUT(base+"/roles/:id", h.updateRole, rest.Secure(h.mw, "updateRole")...)
	root.DELETE(base+"/roles/:id", h.deleteRole, rest.Secure(h.mw, "deleteRole")...)

	root.GET(base+"/roles/:id/permissions", h.getRolePermissions, rest.Secure(h.mw, "getRolePermissions")...)
	root.POST(base+"/roles/:id/permissions", h.addRolePermission, rest.Secure(h.mw, "addRolePermission")...)
	root.PUT(base+"/roles/:id/permissions", h.replaceRolePermissions, rest.Secure(h.mw, "replaceRolePermissions")...)
	root.DELETE(base+"/roles/:id/permissions/:permissionId", h.removeRolePermission, rest.Secure(h.mw, "removeRolePermission")...)

	root.GET(base+"/permissions", h.getAllPermission, rest.Secure(h.mw, "getAllPermission")...)
	root.POST(base+"/permissions", h.createPermission, rest.Secure(h.mw, "createPermission")...)
	root.POST(base+"/permissions/bulk", h.upsertPermissions, rest.Secure(h.mw, "upsertPermissions")...)
	root.GET(base+"/permissions/:id", h.getDetailPermission, rest.Secure(h.mw, "getDetailPermission")...)
	root.PUT(base+"/permissions/:id", h.updatePermission, rest.Secure(h.mw, "updatePermission")...)
	root.DELETE(base+"/permissions/:id", h.deletePermission, rest.Secure(h.mw, "deletePermission")...)
}

func realmParam(req *http.Request) string { return restserver.URLParam(req, "realm") }

// getAllRole godoc
// @Summary		Get All Role of a realm
// @Tags		RBAC
// @Param		realm	path	string	true	"Realm name"
// @Success		200	{object}	domain.ResponseRoleList
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/roles [get]
func (h *RestHandler) getAllRole(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "RbacDeliveryREST:GetAllRole")
	defer trace.Finish()

	var filter domain.FilterRole
	if !rest.ParseFilter(rw, req, h.validator, "rbac/get_all_role", &filter) {
		return
	}
	result, err := h.uc.Rbac().GetAllRole(ctx, realmParam(req), &filter)
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	response := wrapper.NewHTTPResponse(http.StatusOK, "Success", result.Data)
	response.Meta = result.Meta
	response.JSON(rw)
}

// getDetailRole godoc
// @Summary		Get Detail Role (with permissions)
// @Tags		RBAC
// @Param		realm	path	string	true	"Realm name"
// @Param		id	path	int	true	"Role ID"
// @Success		200	{object}	domain.ResponseRole
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/roles/{id} [get]
func (h *RestHandler) getDetailRole(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "RbacDeliveryREST:GetDetailRole")
	defer trace.Finish()

	data, err := h.uc.Rbac().GetDetailRole(ctx, realmParam(req), rest.URLParamInt(req, "id"))
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw, data)
}

// createRole godoc
// @Summary		Create Role
// @Tags		RBAC
// @Accept		json
// @Param		realm	path	string	true	"Realm name"
// @Param		data	body	domain.RequestRole	true	"Body Data"
// @Success		201	{object}	domain.ResponseRole
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/roles [post]
func (h *RestHandler) createRole(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "RbacDeliveryREST:CreateRole")
	defer trace.Finish()

	var payload domain.RequestRole
	if !rest.DecodeBody(rw, req, h.validator, "rbac/save_role", &payload) {
		return
	}
	res, err := h.uc.Rbac().CreateRole(ctx, realmParam(req), &payload)
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	wrapper.NewHTTPResponse(http.StatusCreated, "Success", res).JSON(rw)
}

// updateRole godoc
// @Summary		Update Role
// @Tags		RBAC
// @Accept		json
// @Param		realm	path	string	true	"Realm name"
// @Param		id	path	int	true	"Role ID"
// @Param		data	body	domain.RequestRole	true	"Body Data"
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/roles/{id} [put]
func (h *RestHandler) updateRole(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "RbacDeliveryREST:UpdateRole")
	defer trace.Finish()

	var payload domain.RequestRole
	if !rest.DecodeBody(rw, req, h.validator, "rbac/save_role", &payload) {
		return
	}
	if err := h.uc.Rbac().UpdateRole(ctx, realmParam(req), rest.URLParamInt(req, "id"), &payload); err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw)
}

// deleteRole godoc
// @Summary		Delete Role (409 while assigned to users, unless force=true)
// @Tags		RBAC
// @Param		realm	path	string	true	"Realm name"
// @Param		id	path	int	true	"Role ID"
// @Param		force	query	bool	false	"Also remove user assignments"
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/roles/{id} [delete]
func (h *RestHandler) deleteRole(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "RbacDeliveryREST:DeleteRole")
	defer trace.Finish()

	force := req.URL.Query().Get("force") == "true"
	if err := h.uc.Rbac().DeleteRole(ctx, realmParam(req), rest.URLParamInt(req, "id"), force); err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw)
}

// getRolePermissions godoc
// @Summary		List the permissions of a role
// @Tags		RBAC
// @Param		realm	path	string	true	"Realm name"
// @Param		id	path	int	true	"Role ID"
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/roles/{id}/permissions [get]
func (h *RestHandler) getRolePermissions(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "RbacDeliveryREST:GetRolePermissions")
	defer trace.Finish()

	data, err := h.uc.Rbac().GetRolePermissions(ctx, realmParam(req), rest.URLParamInt(req, "id"))
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw, data)
}

// addRolePermission godoc
// @Summary		Add one permission to a role
// @Tags		RBAC
// @Accept		json
// @Param		realm	path	string	true	"Realm name"
// @Param		id	path	int	true	"Role ID"
// @Param		data	body	domain.RequestPermissionID	true	"Body Data"
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/roles/{id}/permissions [post]
func (h *RestHandler) addRolePermission(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "RbacDeliveryREST:AddRolePermission")
	defer trace.Finish()

	var payload domain.RequestPermissionID
	if !rest.DecodeBody(rw, req, h.validator, "rbac/add_role_permission", &payload) {
		return
	}
	if err := h.uc.Rbac().AddRolePermission(ctx, realmParam(req), rest.URLParamInt(req, "id"), payload.PermissionID); err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw)
}

// replaceRolePermissions godoc
// @Summary		Replace the whole permission set of a role
// @Tags		RBAC
// @Accept		json
// @Param		realm	path	string	true	"Realm name"
// @Param		id	path	int	true	"Role ID"
// @Param		data	body	domain.RequestPermissionIDs	true	"Body Data"
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/roles/{id}/permissions [put]
func (h *RestHandler) replaceRolePermissions(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "RbacDeliveryREST:ReplaceRolePermissions")
	defer trace.Finish()

	var payload domain.RequestPermissionIDs
	if !rest.DecodeBody(rw, req, h.validator, "rbac/replace_role_permissions", &payload) {
		return
	}
	if err := h.uc.Rbac().ReplaceRolePermissions(ctx, realmParam(req), rest.URLParamInt(req, "id"), payload.PermissionIDs); err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw)
}

// removeRolePermission godoc
// @Summary		Remove one permission from a role
// @Tags		RBAC
// @Param		realm	path	string	true	"Realm name"
// @Param		id	path	int	true	"Role ID"
// @Param		permissionId	path	int	true	"Permission ID"
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/roles/{id}/permissions/{permissionId} [delete]
func (h *RestHandler) removeRolePermission(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "RbacDeliveryREST:RemoveRolePermission")
	defer trace.Finish()

	if err := h.uc.Rbac().RemoveRolePermission(ctx, realmParam(req), rest.URLParamInt(req, "id"), rest.URLParamInt(req, "permissionId")); err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw)
}

// getAllPermission godoc
// @Summary		Get All Permission of a realm
// @Tags		RBAC
// @Param		realm	path	string	true	"Realm name"
// @Success		200	{object}	domain.ResponsePermissionList
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/permissions [get]
func (h *RestHandler) getAllPermission(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "RbacDeliveryREST:GetAllPermission")
	defer trace.Finish()

	var filter domain.FilterPermission
	if !rest.ParseFilter(rw, req, h.validator, "rbac/get_all_permission", &filter) {
		return
	}
	result, err := h.uc.Rbac().GetAllPermission(ctx, realmParam(req), &filter)
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	response := wrapper.NewHTTPResponse(http.StatusOK, "Success", result.Data)
	response.Meta = result.Meta
	response.JSON(rw)
}

// getDetailPermission godoc
// @Summary		Get Detail Permission
// @Tags		RBAC
// @Param		realm	path	string	true	"Realm name"
// @Param		id	path	int	true	"Permission ID"
// @Success		200	{object}	domain.ResponsePermission
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/permissions/{id} [get]
func (h *RestHandler) getDetailPermission(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "RbacDeliveryREST:GetDetailPermission")
	defer trace.Finish()

	data, err := h.uc.Rbac().GetDetailPermission(ctx, realmParam(req), rest.URLParamInt(req, "id"))
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw, data)
}

// createPermission godoc
// @Summary		Create Permission
// @Tags		RBAC
// @Accept		json
// @Param		realm	path	string	true	"Realm name"
// @Param		data	body	domain.RequestPermission	true	"Body Data"
// @Success		201	{object}	domain.ResponsePermission
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/permissions [post]
func (h *RestHandler) createPermission(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "RbacDeliveryREST:CreatePermission")
	defer trace.Finish()

	var payload domain.RequestPermission
	if !rest.DecodeBody(rw, req, h.validator, "rbac/save_permission", &payload) {
		return
	}
	res, err := h.uc.Rbac().CreatePermission(ctx, realmParam(req), &payload)
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	wrapper.NewHTTPResponse(http.StatusCreated, "Success", res).JSON(rw)
}

// upsertPermissions godoc
// @Summary		Bulk create-or-update permissions by (service, code)
// @Tags		RBAC
// @Accept		json
// @Param		realm	path	string	true	"Realm name"
// @Param		data	body	domain.RequestPermissionBulk	true	"Body Data"
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/permissions/bulk [post]
func (h *RestHandler) upsertPermissions(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "RbacDeliveryREST:UpsertPermissions")
	defer trace.Finish()

	var payload domain.RequestPermissionBulk
	if !rest.DecodeBody(rw, req, h.validator, "rbac/bulk_permission", &payload) {
		return
	}
	res, err := h.uc.Rbac().UpsertPermissions(ctx, realmParam(req), payload.Permissions)
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw, res)
}

// updatePermission godoc
// @Summary		Update Permission
// @Tags		RBAC
// @Accept		json
// @Param		realm	path	string	true	"Realm name"
// @Param		id	path	int	true	"Permission ID"
// @Param		data	body	domain.RequestPermission	true	"Body Data"
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/permissions/{id} [put]
func (h *RestHandler) updatePermission(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "RbacDeliveryREST:UpdatePermission")
	defer trace.Finish()

	var payload domain.RequestPermission
	if !rest.DecodeBody(rw, req, h.validator, "rbac/save_permission", &payload) {
		return
	}
	if err := h.uc.Rbac().UpdatePermission(ctx, realmParam(req), rest.URLParamInt(req, "id"), &payload); err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw)
}

// deletePermission godoc
// @Summary		Delete Permission (removed from roles, detached from menus)
// @Tags		RBAC
// @Param		realm	path	string	true	"Realm name"
// @Param		id	path	int	true	"Permission ID"
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/permissions/{id} [delete]
func (h *RestHandler) deletePermission(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "RbacDeliveryREST:DeletePermission")
	defer trace.Finish()

	if err := h.uc.Rbac().DeletePermission(ctx, realmParam(req), rest.URLParamInt(req, "id")); err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw)
}
