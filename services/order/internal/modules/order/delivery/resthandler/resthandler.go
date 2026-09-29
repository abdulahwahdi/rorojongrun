package resthandler

import (
	"net/http"

	"monorepo/globalshared/auth"
	"monorepo/globalshared/rest"
	"monorepo/services/order/internal/modules/order/domain"
	shareddomain "monorepo/services/order/pkg/shared/domain"
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
// Orders are never created here: they are booked from the payment service's payment.* events.
func (h *RestHandler) Mount(root interfaces.RESTRouter) {
	root.GET("/v1/orders", h.getAll, rest.Secure(h.mw, "getAllOrders")...)
	root.GET("/v1/orders/summary", h.summary, rest.Secure(h.mw, "getOrderSummary")...)
	root.POST("/v1/orders/quote", h.quote, rest.Secure(h.mw, "quoteOrder")...)
	root.GET("/v1/orders/:id", h.get, rest.Secure(h.mw, "getOrder")...)
	root.PATCH("/v1/orders/:id/status", h.updateStatus, rest.Secure(h.mw, "updateOrderStatus")...)
	root.PATCH("/v1/orders/:id/payment-status", h.overridePaymentStatus, rest.Secure(h.mw, "overridePaymentStatus")...)
}

func (h *RestHandler) getAll(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "OrderDeliveryREST:GetAll")
	defer trace.Finish()

	var filter shareddomain.FilterOrder
	if !rest.ParseFilter(rw, req, h.validator, "order/get_all", &filter) {
		return
	}
	result, err := h.uc.Order().GetAllOrders(ctx, &filter)
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	response := wrapper.NewHTTPResponse(http.StatusOK, "Success", result.Data)
	response.Meta = result.Meta
	response.JSON(rw)
}

func (h *RestHandler) summary(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "OrderDeliveryREST:Summary")
	defer trace.Finish()

	var filter domain.FilterSummary
	if !rest.ParseFilter(rw, req, h.validator, "order/summary", &filter) {
		return
	}
	result, err := h.uc.Order().GetSummary(ctx, &filter)
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw, result)
}

func (h *RestHandler) quote(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "OrderDeliveryREST:Quote")
	defer trace.Finish()

	var payload domain.RequestQuote
	if !rest.DecodeBody(rw, req, h.validator, "order/quote", &payload) {
		return
	}
	result, err := h.uc.Merchant().Quote(ctx, payload.MerchantID, payload.Items)
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw, result)
}

func (h *RestHandler) get(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "OrderDeliveryREST:Get")
	defer trace.Finish()

	result, err := h.uc.Order().GetOrder(ctx, restserver.URLParam(req, "id"))
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw, result)
}

func (h *RestHandler) updateStatus(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "OrderDeliveryREST:UpdateStatus")
	defer trace.Finish()

	var payload domain.RequestUpdateStatus
	if !rest.DecodeBody(rw, req, h.validator, "order/update_status", &payload) {
		return
	}
	result, err := h.uc.Order().UpdateStatus(ctx, restserver.URLParam(req, "id"), auth.SubjectFromContext(ctx), &payload)
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw, result)
}

func (h *RestHandler) overridePaymentStatus(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "OrderDeliveryREST:OverridePaymentStatus")
	defer trace.Finish()

	var payload domain.RequestOverridePaymentStatus
	if !rest.DecodeBody(rw, req, h.validator, "order/override_payment_status", &payload) {
		return
	}
	result, err := h.uc.Order().OverridePaymentStatus(ctx, restserver.URLParam(req, "id"), auth.SubjectFromContext(ctx), &payload)
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw, result)
}
