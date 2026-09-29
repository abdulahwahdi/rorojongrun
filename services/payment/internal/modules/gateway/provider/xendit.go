package provider

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	shareddomain "monorepo/services/payment/pkg/shared/domain"
)

const xenditBase = "https://api.xendit.co"

// Xendit adapter using the Invoice API (hosted page). Credentials: secretKey, callbackToken.
// Settings: baseUrl. A method with channel "invoice" lets the customer choose on Xendit's page,
// any other channel (e.g. "bca", "ovo") restricts the invoice to that payment method.
type Xendit struct{ client *http.Client }

// Code implements Provider
func (x *Xendit) Code() string { return shareddomain.GatewayXendit }

// CreateCharge implements Provider
func (x *Xendit) CreateCharge(ctx context.Context, cfg Config, req ChargeRequest) (ChargeResult, error) {
	secret := cfg.Credentials["secretKey"]
	if secret == "" {
		return ChargeResult{}, errors.New("xendit: secretKey is not configured")
	}
	duration := int(math.Ceil(time.Until(req.ExpiresAt).Seconds()))
	if duration < 60 {
		duration = 60
	}
	items := reconcileItems(req)
	xitems := make([]map[string]any, 0, len(items))
	for _, it := range items {
		xitems = append(xitems, map[string]any{"name": truncate(it.Name, 100), "quantity": it.Quantity, "price": it.Price})
	}
	body := map[string]any{
		"external_id":      req.TransactionID,
		"amount":           req.Amount,
		"currency":         req.Currency,
		"description":      req.Description,
		"invoice_duration": duration,
		"items":            xitems,
	}
	if req.Customer.Email != "" {
		body["payer_email"] = req.Customer.Email
	}
	customer := map[string]any{}
	if req.Customer.Name != "" {
		customer["given_names"] = req.Customer.Name
	}
	if req.Customer.Email != "" {
		customer["email"] = req.Customer.Email
	}
	if req.Customer.Phone != "" {
		customer["mobile_number"] = req.Customer.Phone
	}
	if len(customer) > 0 {
		body["customer"] = customer
	}
	if req.SuccessURL != "" {
		body["success_redirect_url"] = req.SuccessURL
	}
	if req.FailureURL != "" {
		body["failure_redirect_url"] = req.FailureURL
	}
	if ch := req.Method.Channel; ch != "" && ch != "invoice" {
		body["payment_methods"] = []string{strings.ToUpper(ch)}
	}

	var res struct {
		ID         string `json:"id"`
		InvoiceURL string `json:"invoice_url"`
	}
	raw, err := doJSON(ctx, x.client, http.MethodPost, cfg.BaseURL(xenditBase, xenditBase)+"/v2/invoices", secret, body, &res)
	if err != nil {
		return ChargeResult{Raw: raw}, fmt.Errorf("xendit invoice: %w", err)
	}
	if res.InvoiceURL == "" {
		return ChargeResult{Raw: raw}, errors.New("xendit returned no invoice_url")
	}
	return ChargeResult{
		Instruction: map[string]any{"type": "redirect", "url": res.InvoiceURL, "channel": req.Method.Channel},
		ExternalRef: res.ID, Raw: raw,
	}, nil
}

// Cancel implements Provider by expiring the invoice
func (x *Xendit) Cancel(ctx context.Context, cfg Config, _, externalRef string) error {
	secret := cfg.Credentials["secretKey"]
	if secret == "" || externalRef == "" {
		return errors.New("xendit: secretKey or invoice id missing")
	}
	_, err := doJSON(ctx, x.client, http.MethodPost, cfg.BaseURL(xenditBase, xenditBase)+"/invoices/"+externalRef+"/expire!", secret, nil, nil)
	return err
}

// ParseCallback implements Provider. Xendit authenticates with the x-callback-token header.
func (x *Xendit) ParseCallback(_ context.Context, cfg Config, headers map[string]string, body []byte) (CallbackResult, error) {
	token := cfg.Credentials["callbackToken"]
	if token == "" || subtle.ConstantTimeCompare([]byte(headers["x-callback-token"]), []byte(token)) != 1 {
		return CallbackResult{}, fmt.Errorf("%w: callback token mismatch", ErrInvalidCallback)
	}
	var n struct {
		ExternalID    string  `json:"external_id"`
		Status        string  `json:"status"`
		Amount        float64 `json:"amount"`
		PaidAmount    float64 `json:"paid_amount"`
		PaidAt        string  `json:"paid_at"`
		FailureReason string  `json:"failure_reason"`
	}
	if err := json.Unmarshal(body, &n); err != nil || n.ExternalID == "" {
		return CallbackResult{}, fmt.Errorf("%w: unreadable body", ErrInvalidCallback)
	}
	res := CallbackResult{TransactionID: n.ExternalID, Amount: int64(math.Round(n.Amount)), Reason: n.FailureReason}
	switch strings.ToUpper(n.Status) {
	case "PAID", "SETTLED":
		res.Status = CallbackPaid
		if n.PaidAmount > 0 {
			res.Amount = int64(math.Round(n.PaidAmount))
		}
		res.PaidAt = time.Now()
		if t, err := time.Parse(time.RFC3339, n.PaidAt); err == nil {
			res.PaidAt = t
		}
	case "EXPIRED":
		res.Status = CallbackExpired
	default:
		res.Status = CallbackIgnored
	}
	return res, nil
}
