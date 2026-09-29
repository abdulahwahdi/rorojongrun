package resthandler

import (
	"net/http"

	"monorepo/services/user/internal/modules/user/domain"
	"monorepo/services/user/pkg/helper"
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
	base := candihelper.V1 + "/realms/:realm/users"

	root.GET(base, h.getAllUser, helper.Secure(h.mw, "getAllUser")...)
	root.POST(base, h.createUser, helper.Secure(h.mw, "createUser")...)
	root.GET(base+"/:id", h.getDetailUser, helper.Secure(h.mw, "getDetailUser")...)
	root.PUT(base+"/:id", h.updateUser, helper.Secure(h.mw, "updateUser")...)
	root.DELETE(base+"/:id", h.deleteUser, helper.Secure(h.mw, "deleteUser")...)
	root.PUT(base+"/:id/password", h.setPassword, helper.Secure(h.mw, "setUserPassword")...)
	root.POST(base+"/:id/unlock", h.unlockUser, helper.Secure(h.mw, "unlockUser")...)

	root.GET(base+"/:id/roles", h.getUserRoles, helper.Secure(h.mw, "getUserRoles")...)
	root.POST(base+"/:id/roles", h.addUserRole, helper.Secure(h.mw, "addUserRole")...)
	root.PUT(base+"/:id/roles", h.replaceUserRoles, helper.Secure(h.mw, "replaceUserRoles")...)
	root.DELETE(base+"/:id/roles/:roleId", h.removeUserRole, helper.Secure(h.mw, "removeUserRole")...)
}

func realmParam(req *http.Request) string { return restserver.URLParam(req, "realm") }

// getAllUser godoc
// @Summary		Get All User of a realm
// @Tags		User
// @Param		realm	path	string	true	"Realm name"
// @Success		200	{object}	domain.ResponseUserList
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/users [get]
func (h *RestHandler) getAllUser(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "UserDeliveryREST:GetAllUser")
	defer trace.Finish()

	var filter domain.FilterUser
	if !helper.ParseFilter(rw, req, h.validator, "user/get_all", &filter) {
		return
	}
	result, err := h.uc.User().GetAllUser(ctx, realmParam(req), &filter)
	if err != nil {
		helper.WriteError(rw, err)
		return
	}
	response := wrapper.NewHTTPResponse(http.StatusOK, "Success", result.Data)
	response.Meta = result.Meta
	response.JSON(rw)
}

// getDetailUser godoc
// @Summary		Get Detail User (with roles)
// @Tags		User
// @Param		realm	path	string	true	"Realm name"
// @Param		id	path	int	true	"User ID"
// @Success		200	{object}	domain.ResponseUser
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/users/{id} [get]
func (h *RestHandler) getDetailUser(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "UserDeliveryREST:GetDetailUser")
	defer trace.Finish()

	data, err := h.uc.User().GetDetailUser(ctx, realmParam(req), helper.URLParamInt(req, "id"))
	if err != nil {
		helper.WriteError(rw, err)
		return
	}
	helper.WriteOK(rw, data)
}

// createUser godoc
// @Summary		Create User
// @Tags		User
// @Accept		json
// @Param		realm	path	string	true	"Realm name"
// @Param		data	body	domain.RequestCreateUser	true	"Body Data"
// @Success		201	{object}	domain.ResponseUser
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/users [post]
func (h *RestHandler) createUser(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "UserDeliveryREST:CreateUser")
	defer trace.Finish()

	var payload domain.RequestCreateUser
	if !helper.DecodeBody(rw, req, h.validator, "user/create", &payload) {
		return
	}
	res, err := h.uc.User().CreateUser(ctx, realmParam(req), &payload)
	if err != nil {
		helper.WriteError(rw, err)
		return
	}
	wrapper.NewHTTPResponse(http.StatusCreated, "Success", res).JSON(rw)
}

// updateUser godoc
// @Summary		Update User profile / status
// @Tags		User
// @Accept		json
// @Param		realm	path	string	true	"Realm name"
// @Param		id	path	int	true	"User ID"
// @Param		data	body	domain.RequestUpdateUser	true	"Body Data"
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/users/{id} [put]
func (h *RestHandler) updateUser(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "UserDeliveryREST:UpdateUser")
	defer trace.Finish()

	var payload domain.RequestUpdateUser
	if !helper.DecodeBody(rw, req, h.validator, "user/update", &payload) {
		return
	}
	if err := h.uc.User().UpdateUser(ctx, realmParam(req), helper.URLParamInt(req, "id"), &payload); err != nil {
		helper.WriteError(rw, err)
		return
	}
	helper.WriteOK(rw)
}

// deleteUser godoc
// @Summary		Delete User (soft delete, revokes sessions)
// @Tags		User
// @Param		realm	path	string	true	"Realm name"
// @Param		id	path	int	true	"User ID"
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/users/{id} [delete]
func (h *RestHandler) deleteUser(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "UserDeliveryREST:DeleteUser")
	defer trace.Finish()

	if err := h.uc.User().DeleteUser(ctx, realmParam(req), helper.URLParamInt(req, "id")); err != nil {
		helper.WriteError(rw, err)
		return
	}
	helper.WriteOK(rw)
}

// setPassword godoc
// @Summary		Set User password (revokes sessions)
// @Tags		User
// @Accept		json
// @Param		realm	path	string	true	"Realm name"
// @Param		id	path	int	true	"User ID"
// @Param		data	body	domain.RequestPassword	true	"Body Data"
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/users/{id}/password [put]
func (h *RestHandler) setPassword(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "UserDeliveryREST:SetPassword")
	defer trace.Finish()

	var payload domain.RequestPassword
	if !helper.DecodeBody(rw, req, h.validator, "user/password", &payload) {
		return
	}
	if err := h.uc.User().SetPassword(ctx, realmParam(req), helper.URLParamInt(req, "id"), payload.Password); err != nil {
		helper.WriteError(rw, err)
		return
	}
	helper.WriteOK(rw)
}

// unlockUser godoc
// @Summary		Clear the failed-login lockout of a User
// @Tags		User
// @Param		realm	path	string	true	"Realm name"
// @Param		id	path	int	true	"User ID"
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/users/{id}/unlock [post]
func (h *RestHandler) unlockUser(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "UserDeliveryREST:UnlockUser")
	defer trace.Finish()

	if err := h.uc.User().UnlockUser(ctx, realmParam(req), helper.URLParamInt(req, "id")); err != nil {
		helper.WriteError(rw, err)
		return
	}
	helper.WriteOK(rw)
}

// getUserRoles godoc
// @Summary		List the roles of a User
// @Tags		User
// @Param		realm	path	string	true	"Realm name"
// @Param		id	path	int	true	"User ID"
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/users/{id}/roles [get]
func (h *RestHandler) getUserRoles(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "UserDeliveryREST:GetUserRoles")
	defer trace.Finish()

	data, err := h.uc.User().GetUserRoles(ctx, realmParam(req), helper.URLParamInt(req, "id"))
	if err != nil {
		helper.WriteError(rw, err)
		return
	}
	helper.WriteOK(rw, data)
}

// addUserRole godoc
// @Summary		Add one role to a User
// @Tags		User
// @Accept		json
// @Param		realm	path	string	true	"Realm name"
// @Param		id	path	int	true	"User ID"
// @Param		data	body	domain.RequestRoleID	true	"Body Data"
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/users/{id}/roles [post]
func (h *RestHandler) addUserRole(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "UserDeliveryREST:AddUserRole")
	defer trace.Finish()

	var payload domain.RequestRoleID
	if !helper.DecodeBody(rw, req, h.validator, "user/add_role", &payload) {
		return
	}
	if err := h.uc.User().AddUserRole(ctx, realmParam(req), helper.URLParamInt(req, "id"), payload.RoleID); err != nil {
		helper.WriteError(rw, err)
		return
	}
	helper.WriteOK(rw)
}

// replaceUserRoles godoc
// @Summary		Replace the whole role set of a User
// @Tags		User
// @Accept		json
// @Param		realm	path	string	true	"Realm name"
// @Param		id	path	int	true	"User ID"
// @Param		data	body	domain.RequestRoleIDs	true	"Body Data"
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/users/{id}/roles [put]
func (h *RestHandler) replaceUserRoles(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "UserDeliveryREST:ReplaceUserRoles")
	defer trace.Finish()

	var payload domain.RequestRoleIDs
	if !helper.DecodeBody(rw, req, h.validator, "user/replace_roles", &payload) {
		return
	}
	if err := h.uc.User().ReplaceUserRoles(ctx, realmParam(req), helper.URLParamInt(req, "id"), payload.RoleIDs); err != nil {
		helper.WriteError(rw, err)
		return
	}
	helper.WriteOK(rw)
}

// removeUserRole godoc
// @Summary		Remove one role from a User
// @Tags		User
// @Param		realm	path	string	true	"Realm name"
// @Param		id	path	int	true	"User ID"
// @Param		roleId	path	int	true	"Role ID"
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/users/{id}/roles/{roleId} [delete]
func (h *RestHandler) removeUserRole(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "UserDeliveryREST:RemoveUserRole")
	defer trace.Finish()

	if err := h.uc.User().RemoveUserRole(ctx, realmParam(req), helper.URLParamInt(req, "id"), helper.URLParamInt(req, "roleId")); err != nil {
		helper.WriteError(rw, err)
		return
	}
	helper.WriteOK(rw)
}
