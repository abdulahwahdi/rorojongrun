package resthandler

import (
	"net/http"

	"monorepo/globalshared/rest"
	"monorepo/services/user/internal/modules/menu/domain"
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

// Mount handler with root "/". In these routes ":id" is the client_id string of the client app
// (e.g. "backoffice-web"), ":menuId" the numeric menu id. Flat routes, see realm resthandler for why.
func (h *RestHandler) Mount(root interfaces.RESTRouter) {
	base := candihelper.V1 + "/realms/:realm/clients/:id/menus"

	root.GET(base, h.getAllMenu, rest.Secure(h.mw, "getAllMenu")...)
	root.POST(base, h.createMenu, rest.Secure(h.mw, "createMenu")...)
	root.GET(base+"/:menuId", h.getDetailMenu, rest.Secure(h.mw, "getDetailMenu")...)
	root.PUT(base+"/:menuId", h.updateMenu, rest.Secure(h.mw, "updateMenu")...)
	root.DELETE(base+"/:menuId", h.deleteMenu, rest.Secure(h.mw, "deleteMenu")...)
}

func realmParam(req *http.Request) string  { return restserver.URLParam(req, "realm") }
func clientParam(req *http.Request) string { return restserver.URLParam(req, "id") }

// getAllMenu godoc
// @Summary		Get All Menu of a client (flat page, or nested with tree=true)
// @Tags		Menu
// @Param		realm	path	string	true	"Realm name"
// @Param		id	path	string	true	"client_id"
// @Param		tree	query	bool	false	"Return the whole nested tree"
// @Success		200	{object}	domain.ResponseMenuList
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/clients/{id}/menus [get]
func (h *RestHandler) getAllMenu(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "MenuDeliveryREST:GetAllMenu")
	defer trace.Finish()

	var filter domain.FilterMenu
	if !rest.ParseFilter(rw, req, h.validator, "menu/get_all", &filter) {
		return
	}
	result, err := h.uc.Menu().GetAllMenu(ctx, realmParam(req), clientParam(req), &filter)
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	response := wrapper.NewHTTPResponse(http.StatusOK, "Success", result.Data)
	response.Meta = result.Meta
	response.JSON(rw)
}

// getDetailMenu godoc
// @Summary		Get Detail Menu
// @Tags		Menu
// @Param		realm	path	string	true	"Realm name"
// @Param		id	path	string	true	"client_id"
// @Param		menuId	path	int	true	"Menu ID"
// @Success		200	{object}	domain.ResponseMenu
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/clients/{id}/menus/{menuId} [get]
func (h *RestHandler) getDetailMenu(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "MenuDeliveryREST:GetDetailMenu")
	defer trace.Finish()

	data, err := h.uc.Menu().GetDetailMenu(ctx, realmParam(req), clientParam(req), rest.URLParamInt(req, "menuId"))
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw, data)
}

// createMenu godoc
// @Summary		Create Menu
// @Tags		Menu
// @Accept		json
// @Param		realm	path	string	true	"Realm name"
// @Param		id	path	string	true	"client_id"
// @Param		data	body	domain.RequestMenu	true	"Body Data"
// @Success		201	{object}	domain.ResponseMenu
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/clients/{id}/menus [post]
func (h *RestHandler) createMenu(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "MenuDeliveryREST:CreateMenu")
	defer trace.Finish()

	var payload domain.RequestMenu
	if !rest.DecodeBody(rw, req, h.validator, "menu/save", &payload) {
		return
	}
	res, err := h.uc.Menu().CreateMenu(ctx, realmParam(req), clientParam(req), &payload)
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	wrapper.NewHTTPResponse(http.StatusCreated, "Success", res).JSON(rw)
}

// updateMenu godoc
// @Summary		Update Menu (also used to move / reorder)
// @Tags		Menu
// @Accept		json
// @Param		realm	path	string	true	"Realm name"
// @Param		id	path	string	true	"client_id"
// @Param		menuId	path	int	true	"Menu ID"
// @Param		data	body	domain.RequestMenu	true	"Body Data"
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/clients/{id}/menus/{menuId} [put]
func (h *RestHandler) updateMenu(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "MenuDeliveryREST:UpdateMenu")
	defer trace.Finish()

	var payload domain.RequestMenu
	if !rest.DecodeBody(rw, req, h.validator, "menu/save", &payload) {
		return
	}
	if err := h.uc.Menu().UpdateMenu(ctx, realmParam(req), clientParam(req), rest.URLParamInt(req, "menuId"), &payload); err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw)
}

// deleteMenu godoc
// @Summary		Delete Menu (with its descendants)
// @Tags		Menu
// @Param		realm	path	string	true	"Realm name"
// @Param		id	path	string	true	"client_id"
// @Param		menuId	path	int	true	"Menu ID"
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/clients/{id}/menus/{menuId} [delete]
func (h *RestHandler) deleteMenu(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "MenuDeliveryREST:DeleteMenu")
	defer trace.Finish()

	if err := h.uc.Menu().DeleteMenu(ctx, realmParam(req), clientParam(req), rest.URLParamInt(req, "menuId")); err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw)
}
