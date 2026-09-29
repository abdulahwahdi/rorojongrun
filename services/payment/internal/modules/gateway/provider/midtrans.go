package provider

import (
	"context"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	shareddomain "monorepo/services/payment/pkg/shared/domain"
)

const (
	midtransAPISandbox  = "https://api.sandbox.midtrans.com"
	midtransAPIProd     = "https://api.midtrans.com"
	midtransSnapSandbox = "https://app.sandbox.midtrans.com"
	midtransSnapProd    = "https://app.midtrans.com"
)

// Midtrans adapter. Credentials: serverKey. Settings: baseUrl (Core API), snapBaseUrl.
// Virtual accounts / QRIS / e-wallets use the Core API charge, cards use a Snap redirect.
type Midtrans struct{ client *http.Client }

// Code implements Provider
func (m *Midtrans) Code() string { return shareddomain.GatewayMidtrans }

type midtransCharge struct {
	PaymentType        string           `json:"payment_type,omitempty"`
	TransactionDetails map[string]any   `json:"transaction_details"`
	ItemDetails        []map[string]any `json:"item_details"`
	CustomerDetails    map[string]any   `json:"customer_details,omitempty"`
	CustomExpiry       map[string]any   `json:"custom_expiry,omitempty"`
	BankTransfer       map[string]any   `json:"bank_transfer,omitempty"`
	Echannel           map[string]any   `json:"echannel,omitempty"`
	Gopay              map[string]any   `json:"gopay,omitempty"`
	ShopeePay          map[string]any   `json:"shopeepay,omitempty"`
	Qris               map[string]any   `json:"qris,omitempty"`
}

func midtransMinutes(expiresAt time.Time) int {
	mins := int(math.Ceil(time.Until(expiresAt).Minutes()))
	if mins < 1 {
		mins = 1
	}
	return mins
}

func midtransItems(req ChargeRequest) []map[string]any {
	items := reconcileItems(req)
	out := make([]map[string]any, 0, len(items))
	for i, it := range items {
		out = append(out, map[string]any{
			"id": fmt.Sprintf("item-%d", i+1), "price": it.Price, "quantity": it.Quantity, "name": truncate(it.Name, 50),
		})
	}
	return out
}

func midtransCustomer(c shareddomain.Customer) map[string]any {
	out := map[string]any{}
	if c.Name != "" {
		out["first_name"] = c.Name
	}
	if c.Email != "" {
		out["email"] = c.Email
	}
	if c.Phone != "" {
		out["phone"] = c.Phone
	}
	return out
}

// CreateCharge implements Provider
func (m *Midtrans) CreateCharge(ctx context.Context, cfg Config, req ChargeRequest) (ChargeResult, error) {
	serverKey := cfg.Credentials["serverKey"]
	if serverKey == "" {
		return ChargeResult{}, errors.New("midtrans: serverKey is not configured")
	}
	body := midtransCharge{
		TransactionDetails: map[string]any{"order_id": req.TransactionID, "gross_amount": req.Amount},
		ItemDetails:        midtransItems(req),
		CustomerDetails:    midtransCustomer(req.Customer),
	}

	if req.Method.Type == shareddomain.MethodCard {
		return m.snap(ctx, cfg, serverKey, body, req)
	}

	body.CustomExpiry = map[string]any{"expiry_duration": midtransMinutes(req.ExpiresAt), "unit": "minute"}
	channel := req.Method.Channel
	switch {
	case req.Method.Type == shareddomain.MethodVirtualAccount && channel == "mandiri":
		body.PaymentType = "echannel"
		body.Echannel = map[string]any{"bill_info1": "Payment:", "bill_info2": truncate(req.Description, 30)}
	case req.Method.Type == shareddomain.MethodVirtualAccount:
		body.PaymentType = "bank_transfer"
		body.BankTransfer = map[string]any{"bank": channel}
	case channel == "qris":
		body.PaymentType = "qris"
		body.Qris = map[string]any{"acquirer": "gopay"}
	case channel == "gopay":
		body.PaymentType = "gopay"
		body.Gopay = map[string]any{"enable_callback": req.SuccessURL != "", "callback_url": req.SuccessURL}
	case channel == "shopeepay":
		body.PaymentType = "shopeepay"
		body.ShopeePay = map[string]any{"callback_url": req.SuccessURL}
	default:
		return ChargeResult{}, fmt.Errorf("midtrans: unsupported method %s (%s/%s)", req.Method.Code, req.Method.Type, channel)
	}

	var res struct {
		StatusCode    string `json:"status_code"`
		StatusMessage string `json:"status_message"`
		TransactionID string `json:"transaction_id"`
		VANumbers     []struct {
			Bank     string `json:"bank"`
			VANumber string `json:"va_number"`
		} `json:"va_numbers"`
		PermataVANumber string `json:"permata_va_number"`
		BillKey         string `json:"bill_key"`
		BillerCode      string `json:"biller_code"`
		QRString        string `json:"qr_string"`
		Actions         []struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"actions"`
	}
	raw, err := doJSON(ctx, m.client, http.MethodPost, cfg.BaseURL(midtransAPISandbox, midtransAPIProd)+"/v2/charge", serverKey, body, &res)
	if err != nil {
		return ChargeResult{Raw: raw}, fmt.Errorf("midtrans charge: %w", err)
	}
	// Core API answers HTTP 200 with a body status_code
	if res.StatusCode != "201" && res.StatusCode != "200" {
		return ChargeResult{Raw: raw}, fmt.Errorf("midtrans charge rejected: %s %s", res.StatusCode, res.StatusMessage)
	}

	ins := map[string]any{"type": req.Method.Type, "channel": channel}
	switch {
	case len(res.VANumbers) > 0:
		ins["bank"], ins["vaNumber"] = res.VANumbers[0].Bank, res.VANumbers[0].VANumber
	case res.PermataVANumber != "":
		ins["bank"], ins["vaNumber"] = "permata", res.PermataVANumber
	case res.BillKey != "":
		ins["bank"], ins["billerCode"], ins["billKey"] = "mandiri", res.BillerCode, res.BillKey
	}
	if res.QRString != "" {
		ins["qrString"] = res.QRString
	}
	for _, a := range res.Actions {
		switch a.Name {
		case "generate-qr-code", "generate-qr-code-v2":
			ins["qrUrl"] = a.URL
		case "deeplink-redirect":
			ins["deeplinkUrl"] = a.URL
		}
	}
	if len(ins) == 2 {
		return ChargeResult{Raw: raw}, errors.New("midtrans charge returned no payment instruction")
	}
	return ChargeResult{Instruction: ins, ExternalRef: res.TransactionID, Raw: raw}, nil
}

func (m *Midtrans) snap(ctx context.Context, cfg Config, serverKey string, body midtransCharge, req ChargeRequest) (ChargeResult, error) {
	snapBody := map[string]any{
		"transaction_details": body.TransactionDetails,
		"item_details":        body.ItemDetails,
		"customer_details":    body.CustomerDetails,
		"enabled_payments":    []string{"credit_card"},
		"expiry":              map[string]any{"unit": "minute", "duration": midtransMinutes(req.ExpiresAt)},
	}
	if req.SuccessURL != "" {
		snapBody["callbacks"] = map[string]any{"finish": req.SuccessURL}
	}
	base := midtransSnapSandbox
	if cfg.IsProduction() {
		base = midtransSnapProd
	}
	if v, ok := cfg.Settings["snapBaseUrl"].(string); ok && v != "" {
		base = strings.TrimRight(v, "/")
	}
	var res struct {
		Token       string `json:"token"`
		RedirectURL string `json:"redirect_url"`
	}
	raw, err := doJSON(ctx, m.client, http.MethodPost, base+"/snap/v1/transactions", serverKey, snapBody, &res)
	if err != nil {
		return ChargeResult{Raw: raw}, fmt.Errorf("midtrans snap: %w", err)
	}
	if res.RedirectURL == "" {
		return ChargeResult{Raw: raw}, errors.New("midtrans snap returned no redirect_url")
	}
	return ChargeResult{
		Instruction: map[string]any{"type": "redirect", "url": res.RedirectURL, "channel": "credit_card"},
		ExternalRef: req.TransactionID, Raw: raw,
	}, nil
}

// Cancel implements Provider
func (m *Midtrans) Cancel(ctx context.Context, cfg Config, transactionID, _ string) error {
	serverKey := cfg.Credentials["serverKey"]
	if serverKey == "" {
		return errors.New("midtrans: serverKey is not configured")
	}
	_, err := doJSON(ctx, m.client, http.MethodPost, cfg.BaseURL(midtransAPISandbox, midtransAPIProd)+"/v2/"+transactionID+"/cancel", serverKey, nil, nil)
	return err
}

// ParseCallback implements Provider. Body is the Midtrans HTTP notification.
func (m *Midtrans) ParseCallback(_ context.Context, cfg Config, _ map[string]string, body []byte) (CallbackResult, error) {
	var n struct {
		OrderID           string `json:"order_id"`
		StatusCode        string `json:"status_code"`
		GrossAmount       string `json:"gross_amount"`
		SignatureKey      string `json:"signature_key"`
		TransactionStatus string `json:"transaction_status"`
		FraudStatus       string `json:"fraud_status"`
		StatusMessage     string `json:"status_message"`
		SettlementTime    string `json:"settlement_time"`
	}
	if err := json.Unmarshal(body, &n); err != nil {
		return CallbackResult{}, fmt.Errorf("%w: %v", ErrInvalidCallback, err)
	}
	serverKey := cfg.Credentials["serverKey"]
	if serverKey == "" || n.OrderID == "" {
		return CallbackResult{}, fmt.Errorf("%w: missing order_id or serverKey", ErrInvalidCallback)
	}
	sum := sha512.Sum512([]byte(n.OrderID + n.StatusCode + n.GrossAmount + serverKey))
	if subtle.ConstantTimeCompare([]byte(hex.EncodeToString(sum[:])), []byte(strings.ToLower(n.SignatureKey))) != 1 {
		return CallbackResult{}, fmt.Errorf("%w: signature mismatch", ErrInvalidCallback)
	}

	gross, _ := strconv.ParseFloat(n.GrossAmount, 64)
	res := CallbackResult{TransactionID: n.OrderID, Amount: int64(math.Round(gross)), Reason: n.StatusMessage}
	switch n.TransactionStatus {
	case "settlement":
		res.Status = CallbackPaid
	case "capture":
		if n.FraudStatus == "" || n.FraudStatus == "accept" {
			res.Status = CallbackPaid
		} else {
			res.Status = CallbackIgnored
		}
	case "deny", "cancel", "failure":
		res.Status, res.Reason = CallbackFailed, "midtrans: "+n.TransactionStatus
	case "expire":
		res.Status = CallbackExpired
	default: // pending, refund, chargeback, ...
		res.Status = CallbackIgnored
	}
	if res.Status == CallbackPaid {
		res.PaidAt = time.Now()
		if t, err := time.ParseInLocation("2006-01-02 15:04:05", n.SettlementTime, time.FixedZone("WIB", 7*3600)); err == nil {
			res.PaidAt = t
		}
	}
	return res, nil
}
