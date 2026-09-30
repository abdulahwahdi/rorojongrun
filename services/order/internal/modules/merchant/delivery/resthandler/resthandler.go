package resthandler

import (
	"net/http"

	"monorepo/globalshared/rest"
	"monorepo/services/order/internal/modules/merchant/domain"
	"monorepo/services/order/pkg/shared/usecase"

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
	return &RestHandler{uc: uc, mw: deps.GetMiddleware(), validator: deps.GetValidator()}
}

// Mount handler with root "/". Routes are registered flat, see the user service for why.
// The merchant id "*" is the default settings row.
func (h *RestHandler) Mount(root interfaces.RESTRouter) {
	root.GET("/v1/merchant-settings", h.getAll, rest.Secure(h.mw, "getMerchantSettings")...)
	root.GET("/v1/merchant-settings/:merchantId", h.get, rest.Secure(h.mw, "getMerchantSettings")...)
	root.PUT("/v1/merchant-settings/:merchantId", h.save, rest.Secure(h.mw, "manageMerchantSettings")...)
	root.DELETE("/v1/merchant-settings/:merchantId", h.delete, rest.Secure(h.mw, "manageMerchantSettings")...)
}

func (h *RestHandler) getAll(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "MerchantDeliveryREST:GetAll")
	defer trace.Finish()

	var filter domain.FilterMerchant
	if !rest.ParseFilter(rw, req, h.validator, "merchant/get_all", &filter) {
		return
	}
	result, err := h.uc.Merchant().GetAllMerchants(ctx, &filter)
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	response := wrapper.NewHTTPResponse(http.StatusOK, "Success", result.Data)
	response.Meta = result.Meta
	response.JSON(rw)
}

func (h *RestHandler) get(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "MerchantDeliveryREST:Get")
	defer trace.Finish()

	data, err := h.uc.Merchant().GetMerchant(ctx, restserver.URLParam(req, "merchantId"))
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw, data)
}

func (h *RestHandler) save(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "MerchantDeliveryREST:Save")
	defer trace.Finish()

	var payload domain.RequestSaveMerchant
	if !rest.DecodeBody(rw, req, h.validator, "merchant/save", &payload) {
		return
	}
	data, err := h.uc.Merchant().SaveMerchant(ctx, restserver.URLParam(req, "merchantId"), &payload)
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw, data)
}

func (h *RestHandler) delete(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "MerchantDeliveryREST:Delete")
	defer trace.Finish()

	if err := h.uc.Merchant().DeleteMerchant(ctx, restserver.URLParam(req, "merchantId")); err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw)
}
