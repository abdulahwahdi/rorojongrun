package resthandler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"monorepo/globalshared/auth"
	"monorepo/services/payment/internal/modules/payment/domain"
	"monorepo/services/payment/pkg/helper"
	"monorepo/services/payment/pkg/shared/usecase"

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
//
//   - /v1/payments*, /v1/cash*, /v1/callback-logs*, /v1/dev/*: other services / staff / operators, bearer token + permission
//   - /v1/checkout/:token*: the customer's checkout journey, the unguessable token is the credential
func (h *RestHandler) Mount(root interfaces.RESTRouter) {
	root.POST("/v1/payments", h.createPayment, helper.Secure(h.mw, "createPayment")...)
	root.GET("/v1/payments", h.getAllPayments, helper.Secure(h.mw, "getAllPayments")...)
	root.GET("/v1/payments/:id", h.getPayment, helper.Secure(h.mw, "getPayment")...)
	root.POST("/v1/payments/:id/cancel", h.cancelPayment, helper.Secure(h.mw, "cancelPayment")...)

	root.GET("/v1/checkout/:token", h.getCheckout)
	root.GET("/v1/checkout/:token/methods", h.getCheckoutMethods)
	root.PUT("/v1/checkout/:token/method", h.selectMethod)
	root.POST("/v1/checkout/:token/pay", h.pay)
	root.GET("/v1/checkout/:token/status", h.getCheckoutStatus)
	root.POST("/v1/checkout/:token/cancel", h.cancelCheckout)

	root.GET("/v1/cash/:cashCode", h.getCash, helper.Secure(h.mw, "confirmCashPayment")...)
	root.POST("/v1/cash/:cashCode/confirm", h.confirmCash, helper.Secure(h.mw, "confirmCashPayment")...)

	root.GET("/v1/callback-logs", h.getAllCallbackLogs, helper.Secure(h.mw, "getCallbackLogs")...)
	root.GET("/v1/callback-logs/:id", h.getCallbackLog, helper.Secure(h.mw, "getCallbackLogs")...)
	root.POST("/v1/callback-logs/:id/replay", h.replayCallback, helper.Secure(h.mw, "replayCallback")...)

	root.POST("/v1/dev/mock-callback", h.mockCallback, helper.Secure(h.mw, "manageGateways")...)
}

func (h *RestHandler) createPayment(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "PaymentDeliveryREST:CreatePayment")
	defer trace.Finish()

	var payload domain.RequestCreatePayment
	if !helper.DecodeBody(rw, req, h.validator, "payment/create", &payload) {
		return
	}
	// A service token (client credentials) is pinned to its own client id, so a service cannot
	// create payments in another service's namespace. An operator (user) token names the source.
	claim := auth.TokenClaimFromContext(ctx)
	source := payload.Source
	if auth.TypeFromClaim(claim) == auth.TypeService || source == "" {
		source = auth.ClientIDFromClaim(claim)
	}
	res, err := h.uc.Payment().CreatePayment(ctx, source, &payload)
	if err != nil {
		helper.WriteError(rw, err)
		return
	}
	wrapper.NewHTTPResponse(http.StatusCreated, "Success", res).JSON(rw)
}

func (h *RestHandler) getAllPayments(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "PaymentDeliveryREST:GetAllPayments")
	defer trace.Finish()

	var filter domain.FilterPayment
	if !helper.ParseFilter(rw, req, h.validator, "payment/get_all", &filter) {
		return
	}
	result, err := h.uc.Payment().GetAllPayments(ctx, &filter)
	if err != nil {
		helper.WriteError(rw, err)
		return
	}
	response := wrapper.NewHTTPResponse(http.StatusOK, "Success", result.Data)
	response.Meta = result.Meta
	response.JSON(rw)
}

func (h *RestHandler) getPayment(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "PaymentDeliveryREST:GetPayment")
	defer trace.Finish()

	res, err := h.uc.Payment().GetPayment(ctx, restserver.URLParam(req, "id"))
	if err != nil {
		helper.WriteError(rw, err)
		return
	}
	helper.WriteOK(rw, res)
}

func (h *RestHandler) cancelPayment(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "PaymentDeliveryREST:CancelPayment")
	defer trace.Finish()

	res, err := h.uc.Payment().CancelPayment(ctx, restserver.URLParam(req, "id"))
	if err != nil {
		helper.WriteError(rw, err)
		return
	}
	helper.WriteOK(rw, res)
}

// ---- checkout

func (h *RestHandler) getCheckout(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "PaymentDeliveryREST:GetCheckout")
	defer trace.Finish()

	res, err := h.uc.Payment().GetCheckout(ctx, restserver.URLParam(req, "token"))
	if err != nil {
		helper.WriteError(rw, err)
		return
	}
	helper.WriteOK(rw, res)
}

func (h *RestHandler) getCheckoutMethods(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "PaymentDeliveryREST:GetCheckoutMethods")
	defer trace.Finish()

	res, err := h.uc.Payment().GetCheckoutMethods(ctx, restserver.URLParam(req, "token"))
	if err != nil {
		helper.WriteError(rw, err)
		return
	}
	helper.WriteOK(rw, res)
}

func (h *RestHandler) selectMethod(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "PaymentDeliveryREST:SelectMethod")
	defer trace.Finish()

	var payload domain.RequestSelectMethod
	if !helper.DecodeBody(rw, req, h.validator, "payment/select_method", &payload) {
		return
	}
	res, err := h.uc.Payment().SelectMethod(ctx, restserver.URLParam(req, "token"), payload.MethodCode)
	if err != nil {
		helper.WriteError(rw, err)
		return
	}
	helper.WriteOK(rw, res)
}

func (h *RestHandler) pay(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "PaymentDeliveryREST:Pay")
	defer trace.Finish()

	// the body is optional: {"methodCode": "..."} selects and pays in one call
	var payload domain.RequestSelectMethod
	if body, _ := io.ReadAll(req.Body); len(bytes.TrimSpace(body)) > 0 {
		if err := h.validator.ValidateDocument("payment/pay", body); err != nil {
			wrapper.NewHTTPResponse(http.StatusBadRequest, "Failed validate payload", err).JSON(rw)
			return
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			wrapper.NewHTTPResponse(http.StatusBadRequest, err.Error()).JSON(rw)
			return
		}
	}
	res, err := h.uc.Payment().Pay(ctx, restserver.URLParam(req, "token"), payload.MethodCode)
	if err != nil {
		helper.WriteError(rw, err)
		return
	}
	helper.WriteOK(rw, res)
}

// getCheckoutStatus is the lightweight poll target of the checkout page
func (h *RestHandler) getCheckoutStatus(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "PaymentDeliveryREST:GetCheckoutStatus")
	defer trace.Finish()

	res, err := h.uc.Payment().GetCheckout(ctx, restserver.URLParam(req, "token"))
	if err != nil {
		helper.WriteError(rw, err)
		return
	}
	helper.WriteOK(rw, map[string]any{
		"status": res.Status, "paidAt": res.PaidAt, "successUrl": res.SuccessURL, "failureUrl": res.FailureURL,
	})
}

func (h *RestHandler) cancelCheckout(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "PaymentDeliveryREST:CancelCheckout")
	defer trace.Finish()

	res, err := h.uc.Payment().CancelCheckout(ctx, restserver.URLParam(req, "token"))
	if err != nil {
		helper.WriteError(rw, err)
		return
	}
	helper.WriteOK(rw, res)
}

// ---- cash

func (h *RestHandler) getCash(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "PaymentDeliveryREST:GetCash")
	defer trace.Finish()

	res, err := h.uc.Payment().GetCashPayment(ctx, restserver.URLParam(req, "cashCode"))
	if err != nil {
		helper.WriteError(rw, err)
		return
	}
	helper.WriteOK(rw, res)
}

func (h *RestHandler) confirmCash(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "PaymentDeliveryREST:ConfirmCash")
	defer trace.Finish()

	var payload domain.RequestConfirmCash
	if !helper.DecodeBody(rw, req, h.validator, "payment/confirm_cash", &payload) {
		return
	}
	actor := ""
	if claim := auth.TokenClaimFromContext(ctx); claim != nil {
		actor = claim.Subject
	}
	res, err := h.uc.Payment().ConfirmCashPayment(ctx, restserver.URLParam(req, "cashCode"), actor, &payload)
	if err != nil {
		helper.WriteError(rw, err)
		return
	}
	helper.WriteOK(rw, res)
}

// ---- callbacks & dev

func (h *RestHandler) getAllCallbackLogs(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "PaymentDeliveryREST:GetAllCallbackLogs")
	defer trace.Finish()

	var filter domain.FilterCallbackLog
	if !helper.ParseFilter(rw, req, h.validator, "payment/get_all_callback_log", &filter) {
		return
	}
	result, err := h.uc.Payment().GetAllCallbackLogs(ctx, &filter)
	if err != nil {
		helper.WriteError(rw, err)
		return
	}
	response := wrapper.NewHTTPResponse(http.StatusOK, "Success", result.Data)
	response.Meta = result.Meta
	response.JSON(rw)
}

func (h *RestHandler) getCallbackLog(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "PaymentDeliveryREST:GetCallbackLog")
	defer trace.Finish()

	res, err := h.uc.Payment().GetCallbackLog(ctx, helper.URLParamInt(req, "id"))
	if err != nil {
		helper.WriteError(rw, err)
		return
	}
	helper.WriteOK(rw, res)
}

func (h *RestHandler) replayCallback(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "PaymentDeliveryREST:ReplayCallback")
	defer trace.Finish()

	out, err := h.uc.Payment().ReplayCallback(ctx, helper.URLParamInt(req, "id"))
	if err != nil {
		helper.WriteError(rw, err)
		return
	}
	helper.WriteOK(rw, map[string]any{"status": out.Status, "transactionId": out.TransactionID, "message": out.Message})
}

func (h *RestHandler) mockCallback(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "PaymentDeliveryREST:MockCallback")
	defer trace.Finish()

	var payload domain.RequestMockCallback
	if !helper.DecodeBody(rw, req, h.validator, "payment/mock_callback", &payload) {
		return
	}
	if err := h.uc.Payment().SimulateMockCallback(ctx, &payload); err != nil {
		helper.WriteError(rw, err)
		return
	}
	wrapper.NewHTTPResponse(http.StatusAccepted, "callback published to Kafka").JSON(rw)
}
