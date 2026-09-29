package resthandler

import (
	"net/http"
	"strconv"

	"monorepo/globalshared/auth"
	"monorepo/globalshared/rest"
	"monorepo/services/order/internal/modules/invoice/domain"
	shareddomain "monorepo/services/order/pkg/shared/domain"
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
// Invoice numbers contain "/", so routes take the numeric id; /lookup?number= finds one by number.
func (h *RestHandler) Mount(root interfaces.RESTRouter) {
	root.GET("/v1/invoices", h.getAll, rest.Secure(h.mw, "getAllInvoices")...)
	root.GET("/v1/invoices/lookup", h.lookup, rest.Secure(h.mw, "getInvoice")...)
	root.GET("/v1/invoices/:id", h.get, rest.Secure(h.mw, "getInvoice")...)
	root.GET("/v1/invoices/:id/print", h.print, rest.Secure(h.mw, "getInvoice")...)
	root.POST("/v1/invoices/:id/resend", h.resend, rest.Secure(h.mw, "resendInvoice")...)
}

func (h *RestHandler) getAll(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "InvoiceDeliveryREST:GetAll")
	defer trace.Finish()

	var filter shareddomain.FilterInvoice
	if !rest.ParseFilter(rw, req, h.validator, "invoice/get_all", &filter) {
		return
	}
	if _, err := h.uc.Order().ResolveDates(ctx, filter.MerchantID, &filter.DateRange); err != nil {
		rest.WriteError(rw, err)
		return
	}
	result, err := h.uc.Invoice().GetAllInvoices(ctx, &filter)
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	response := wrapper.NewHTTPResponse(http.StatusOK, "Success", result.Data)
	response.Meta = result.Meta
	response.JSON(rw)
}

func (h *RestHandler) lookup(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "InvoiceDeliveryREST:Lookup")
	defer trace.Finish()

	number := req.URL.Query().Get("number")
	if number == "" {
		rest.WriteError(rw, rest.NewInvalid("number is required"))
		return
	}
	result, err := h.uc.Invoice().GetInvoiceByNumber(ctx, number)
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw, result)
}

func (h *RestHandler) get(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "InvoiceDeliveryREST:Get")
	defer trace.Finish()

	result, err := h.uc.Invoice().GetInvoice(ctx, int64(rest.URLParamInt(req, "id")))
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw, result)
}

func (h *RestHandler) print(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "InvoiceDeliveryREST:Print")
	defer trace.Finish()

	inv, err := h.uc.Invoice().GetInvoice(ctx, int64(rest.URLParamInt(req, "id")))
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	format := req.URL.Query().Get("format")
	if format != domain.FormatReceipt {
		format = domain.FormatA4
	}
	var refNumber string
	if inv.RefInvoiceID != nil {
		if ref, err := h.uc.Invoice().GetInvoice(ctx, *inv.RefInvoiceID); err == nil {
			refNumber = ref.Number
		}
	}
	m, err := h.uc.Merchant().MerchantSettings(ctx, inv.MerchantID)
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	page, err := renderInvoice(format, inv, refNumber, m.Location())
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	rw.Header().Set("Content-Length", strconv.Itoa(len(page)))
	rw.WriteHeader(http.StatusOK)
	_, _ = rw.Write(page)
}

func (h *RestHandler) resend(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "InvoiceDeliveryREST:Resend")
	defer trace.Finish()

	result, err := h.uc.Invoice().ResendInvoice(ctx, int64(rest.URLParamInt(req, "id")), auth.SubjectFromContext(ctx))
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw, result)
}
