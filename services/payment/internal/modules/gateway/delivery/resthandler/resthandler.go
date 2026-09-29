package resthandler

import (
	"net/http"

	"monorepo/globalshared/rest"
	"monorepo/services/payment/internal/modules/gateway/domain"
	"monorepo/services/payment/pkg/shared/usecase"

	restserver "github.com/golangid/candi/codebase/app/rest_server"
	"github.com/golangid/candi/codebase/factory/dependency"
	"github.com/golangid/candi/codebase/interfaces"
	"github.com/golangid/candi/tracer"
)

// RestHandler handler
type RestHandler struct {
	mw        interfaces.Middleware
	uc        usecase.Usecase
	validator interfaces.Validator
}

// NewRestHandler create new rest handler
func NewRestHandler(uc usecase.Usecase, deps dependency.Dependency) *RestHandler {
	return &RestHandler{uc: uc, mw: deps.GetMiddleware(), validator: deps.GetValidator()}
}

// Mount handler with root "/". Routes are registered flat, see the user service for why.
func (h *RestHandler) Mount(root interfaces.RESTRouter) {
	root.GET("/v1/gateways", h.getAll, rest.Secure(h.mw, "manageGateways")...)
	root.GET("/v1/gateways/:code", h.get, rest.Secure(h.mw, "manageGateways")...)
	root.PUT("/v1/gateways/:code", h.update, rest.Secure(h.mw, "manageGateways")...)
	root.PATCH("/v1/gateways/:code/status", h.setStatus, rest.Secure(h.mw, "manageGateways")...)
}

func (h *RestHandler) getAll(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "GatewayDeliveryREST:GetAll")
	defer trace.Finish()

	data, err := h.uc.Gateway().GetAllGateways(ctx)
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw, data)
}

func (h *RestHandler) get(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "GatewayDeliveryREST:Get")
	defer trace.Finish()

	data, err := h.uc.Gateway().GetGateway(ctx, restserver.URLParam(req, "code"))
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw, data)
}

func (h *RestHandler) update(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "GatewayDeliveryREST:Update")
	defer trace.Finish()

	var payload domain.RequestUpdateGateway
	if !rest.DecodeBody(rw, req, h.validator, "gateway/save", &payload) {
		return
	}
	data, err := h.uc.Gateway().UpdateGateway(ctx, restserver.URLParam(req, "code"), &payload)
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw, data)
}

func (h *RestHandler) setStatus(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "GatewayDeliveryREST:SetStatus")
	defer trace.Finish()

	var payload domain.RequestSetStatus
	if !rest.DecodeBody(rw, req, h.validator, "gateway/status", &payload) {
		return
	}
	data, err := h.uc.Gateway().SetGatewayStatus(ctx, restserver.URLParam(req, "code"), payload.IsEnabled)
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw, data)
}
