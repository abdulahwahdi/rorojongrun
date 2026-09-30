package resthandler

import (
	"net/http"

	"monorepo/globalshared/auth"
	"monorepo/globalshared/rest"
	"monorepo/services/order/internal/modules/shift/domain"
	"monorepo/services/order/pkg/shared/usecase"

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
	root.GET("/v1/shifts", h.getAll, rest.Secure(h.mw, "getShifts")...)
	root.POST("/v1/shifts/open", h.open, rest.Secure(h.mw, "openShift")...)
	root.GET("/v1/shifts/current", h.current, rest.Secure(h.mw, "getShifts")...)
	root.GET("/v1/shifts/:id", h.get, rest.Secure(h.mw, "getShifts")...)
	root.POST("/v1/shifts/:id/close", h.close, rest.Secure(h.mw, "closeShift")...)
}

func (h *RestHandler) getAll(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "ShiftDeliveryREST:GetAll")
	defer trace.Finish()

	var filter domain.FilterShift
	if !rest.ParseFilter(rw, req, h.validator, "shift/get_all", &filter) {
		return
	}
	result, err := h.uc.Shift().GetAllShifts(ctx, &filter)
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	response := wrapper.NewHTTPResponse(http.StatusOK, "Success", result.Data)
	response.Meta = result.Meta
	response.JSON(rw)
}

func (h *RestHandler) open(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "ShiftDeliveryREST:Open")
	defer trace.Finish()

	var payload domain.RequestOpenShift
	if !rest.DecodeBody(rw, req, h.validator, "shift/open", &payload) {
		return
	}
	result, err := h.uc.Shift().OpenShift(ctx, auth.SubjectFromContext(ctx), &payload)
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	wrapper.NewHTTPResponse(http.StatusCreated, "Success", result).JSON(rw)
}

func (h *RestHandler) current(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "ShiftDeliveryREST:Current")
	defer trace.Finish()

	q := req.URL.Query()
	cashier := q.Get("cashierId")
	if cashier == "" {
		cashier = auth.SubjectFromContext(ctx)
	}
	result, err := h.uc.Shift().CurrentShift(ctx, q.Get("merchantId"), q.Get("outletId"), cashier)
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw, result)
}

func (h *RestHandler) get(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "ShiftDeliveryREST:Get")
	defer trace.Finish()

	result, err := h.uc.Shift().GetShift(ctx, int64(rest.URLParamInt(req, "id")))
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw, result)
}

func (h *RestHandler) close(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "ShiftDeliveryREST:Close")
	defer trace.Finish()

	var payload domain.RequestCloseShift
	if !rest.DecodeBody(rw, req, h.validator, "shift/close", &payload) {
		return
	}
	result, err := h.uc.Shift().CloseShift(ctx, int64(rest.URLParamInt(req, "id")), auth.SubjectFromContext(ctx), &payload)
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw, result)
}
