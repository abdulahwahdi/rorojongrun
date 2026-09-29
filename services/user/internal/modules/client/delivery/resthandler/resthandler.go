package resthandler

import (
	"net/http"

	"monorepo/services/user/internal/modules/client/domain"
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
	base := candihelper.V1 + "/realms/:realm/clients"

	root.GET(base, h.getAllClient, helper.Secure(h.mw, "getAllClient")...)
	root.POST(base, h.createClient, helper.Secure(h.mw, "createClient")...)
	root.GET(base+"/:id", h.getDetailClient, helper.Secure(h.mw, "getDetailClient")...)
	root.PUT(base+"/:id", h.updateClient, helper.Secure(h.mw, "updateClient")...)
	root.DELETE(base+"/:id", h.deleteClient, helper.Secure(h.mw, "deleteClient")...)
	root.POST(base+"/:id/secret/rotate", h.rotateClientSecret, helper.Secure(h.mw, "rotateClientSecret")...)
}

func realmParam(req *http.Request) string { return restserver.URLParam(req, "realm") }

// getAllClient godoc
// @Summary		Get All Client of a realm
// @Tags		Client
// @Param		realm	path	string	true	"Realm name"
// @Success		200	{object}	domain.ResponseClientList
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/clients [get]
func (h *RestHandler) getAllClient(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "ClientDeliveryREST:GetAllClient")
	defer trace.Finish()

	var filter domain.FilterClient
	if !helper.ParseFilter(rw, req, h.validator, "client/get_all", &filter) {
		return
	}
	result, err := h.uc.Client().GetAllClient(ctx, realmParam(req), &filter)
	if err != nil {
		helper.WriteError(rw, err)
		return
	}
	response := wrapper.NewHTTPResponse(http.StatusOK, "Success", result.Data)
	response.Meta = result.Meta
	response.JSON(rw)
}

// getDetailClient godoc
// @Summary		Get Detail Client
// @Tags		Client
// @Param		realm	path	string	true	"Realm name"
// @Param		id	path	int	true	"Client ID"
// @Success		200	{object}	domain.ResponseClient
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/clients/{id} [get]
func (h *RestHandler) getDetailClient(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "ClientDeliveryREST:GetDetailClient")
	defer trace.Finish()

	data, err := h.uc.Client().GetDetailClient(ctx, realmParam(req), helper.URLParamInt(req, "id"))
	if err != nil {
		helper.WriteError(rw, err)
		return
	}
	helper.WriteOK(rw, data)
}

// createClient godoc
// @Summary		Create Client (the secret of a confidential client is returned once)
// @Tags		Client
// @Accept		json
// @Param		realm	path	string	true	"Realm name"
// @Param		data	body	domain.RequestCreateClient	true	"Body Data"
// @Success		201	{object}	domain.ResponseClient
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/clients [post]
func (h *RestHandler) createClient(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "ClientDeliveryREST:CreateClient")
	defer trace.Finish()

	var payload domain.RequestCreateClient
	if !helper.DecodeBody(rw, req, h.validator, "client/create", &payload) {
		return
	}
	res, err := h.uc.Client().CreateClient(ctx, realmParam(req), &payload)
	if err != nil {
		helper.WriteError(rw, err)
		return
	}
	wrapper.NewHTTPResponse(http.StatusCreated, "Success", res).JSON(rw)
}

// updateClient godoc
// @Summary		Update Client
// @Tags		Client
// @Accept		json
// @Param		realm	path	string	true	"Realm name"
// @Param		id	path	int	true	"Client ID"
// @Param		data	body	domain.RequestUpdateClient	true	"Body Data"
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/clients/{id} [put]
func (h *RestHandler) updateClient(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "ClientDeliveryREST:UpdateClient")
	defer trace.Finish()

	var payload domain.RequestUpdateClient
	if !helper.DecodeBody(rw, req, h.validator, "client/update", &payload) {
		return
	}
	if err := h.uc.Client().UpdateClient(ctx, realmParam(req), helper.URLParamInt(req, "id"), &payload); err != nil {
		helper.WriteError(rw, err)
		return
	}
	helper.WriteOK(rw)
}

// deleteClient godoc
// @Summary		Delete Client (revokes its sessions, deletes its menus and service account)
// @Tags		Client
// @Param		realm	path	string	true	"Realm name"
// @Param		id	path	int	true	"Client ID"
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/clients/{id} [delete]
func (h *RestHandler) deleteClient(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "ClientDeliveryREST:DeleteClient")
	defer trace.Finish()

	if err := h.uc.Client().DeleteClient(ctx, realmParam(req), helper.URLParamInt(req, "id")); err != nil {
		helper.WriteError(rw, err)
		return
	}
	helper.WriteOK(rw)
}

// rotateClientSecret godoc
// @Summary		Rotate the secret of a confidential Client (new secret returned once)
// @Tags		Client
// @Param		realm	path	string	true	"Realm name"
// @Param		id	path	int	true	"Client ID"
// @Success		200	{object}	domain.ResponseClient
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/clients/{id}/secret/rotate [post]
func (h *RestHandler) rotateClientSecret(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "ClientDeliveryREST:RotateClientSecret")
	defer trace.Finish()

	res, err := h.uc.Client().RotateClientSecret(ctx, realmParam(req), helper.URLParamInt(req, "id"))
	if err != nil {
		helper.WriteError(rw, err)
		return
	}
	helper.WriteOK(rw, res)
}
