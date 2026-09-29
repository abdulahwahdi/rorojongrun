package resthandler

import (
	"net/http"

	"monorepo/services/payment/internal/modules/method/domain"
	"monorepo/services/payment/pkg/helper"
	"monorepo/services/payment/pkg/shared/usecase"

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
	return &RestHandler{uc: uc, mw: deps.GetMiddleware(), validator: deps.GetValidator()}
}

// Mount handler with root "/". Routes are registered flat, see the user service for why.
func (h *RestHandler) Mount(root interfaces.RESTRouter) {
	root.GET("/v1/payment-methods", h.getAll, helper.Secure(h.mw, "manageMethods")...)
	root.POST("/v1/payment-methods", h.create, helper.Secure(h.mw, "manageMethods")...)
	root.GET("/v1/payment-methods/:id", h.get, helper.Secure(h.mw, "manageMethods")...)
	root.PUT("/v1/payment-methods/:id", h.update, helper.Secure(h.mw, "manageMethods")...)
	root.PATCH("/v1/payment-methods/:id/status", h.setStatus, helper.Secure(h.mw, "manageMethods")...)
	root.DELETE("/v1/payment-methods/:id", h.delete, helper.Secure(h.mw, "manageMethods")...)
}

func (h *RestHandler) getAll(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "MethodDeliveryREST:GetAll")
	defer trace.Finish()

	var filter domain.FilterMethod
	if !helper.ParseFilter(rw, req, h.validator, "method/get_all", &filter) {
		return
	}
	result, err := h.uc.Method().GetAllMethods(ctx, &filter)
	if err != nil {
		helper.WriteError(rw, err)
		return
	}
	response := wrapper.NewHTTPResponse(http.StatusOK, "Success", result.Data)
	response.Meta = result.Meta
	response.JSON(rw)
}

func (h *RestHandler) get(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "MethodDeliveryREST:Get")
	defer trace.Finish()

	data, err := h.uc.Method().GetMethod(ctx, helper.URLParamInt(req, "id"))
	if err != nil {
		helper.WriteError(rw, err)
		return
	}
	helper.WriteOK(rw, data)
}

func (h *RestHandler) create(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "MethodDeliveryREST:Create")
	defer trace.Finish()

	var payload domain.RequestSaveMethod
	if !helper.DecodeBody(rw, req, h.validator, "method/save", &payload) {
		return
	}
	data, err := h.uc.Method().CreateMethod(ctx, &payload)
	if err != nil {
		helper.WriteError(rw, err)
		return
	}
	wrapper.NewHTTPResponse(http.StatusCreated, "Success", data).JSON(rw)
}

func (h *RestHandler) update(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "MethodDeliveryREST:Update")
	defer trace.Finish()

	var payload domain.RequestSaveMethod
	if !helper.DecodeBody(rw, req, h.validator, "method/save", &payload) {
		return
	}
	data, err := h.uc.Method().UpdateMethod(ctx, helper.URLParamInt(req, "id"), &payload)
	if err != nil {
		helper.WriteError(rw, err)
		return
	}
	helper.WriteOK(rw, data)
}

func (h *RestHandler) setStatus(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "MethodDeliveryREST:SetStatus")
	defer trace.Finish()

	var payload domain.RequestSetStatus
	if !helper.DecodeBody(rw, req, h.validator, "method/status", &payload) {
		return
	}
	data, err := h.uc.Method().SetMethodStatus(ctx, helper.URLParamInt(req, "id"), payload.IsEnabled)
	if err != nil {
		helper.WriteError(rw, err)
		return
	}
	helper.WriteOK(rw, data)
}

func (h *RestHandler) delete(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "MethodDeliveryREST:Delete")
	defer trace.Finish()

	if err := h.uc.Method().DeleteMethod(ctx, helper.URLParamInt(req, "id")); err != nil {
		helper.WriteError(rw, err)
		return
	}
	helper.WriteOK(rw)
}
