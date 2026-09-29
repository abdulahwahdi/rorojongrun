// Package provider holds the payment gateway adapters. Every gateway is hidden
// behind Provider so the payment module never knows which gateway it talks to.
package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"monorepo/globalshared/crypto"
	shareddomain "monorepo/services/payment/pkg/shared/domain"
)

// Callback statuses returned by ParseCallback
const (
	CallbackPaid    = "paid"
	CallbackFailed  = "failed"
	CallbackExpired = "expired"
	// CallbackIgnored covers pending/refund/other notifications that must not change state
	CallbackIgnored = "ignored"
)

// ErrInvalidCallback means the callback failed signature/token verification or could not be parsed
var ErrInvalidCallback = errors.New("invalid gateway callback")

// Config is the runtime configuration of one gateway: the DB row with credentials decrypted
type Config struct {
	Code        string
	Environment string
	Settings    map[string]any
	Credentials map[string]string
}

// IsProduction reports the gateway environment
func (c Config) IsProduction() bool { return c.Environment == shareddomain.EnvProduction }

// BaseURL returns settings.baseUrl if set, otherwise the sandbox/production default
func (c Config) BaseURL(sandbox, production string) string {
	if v, ok := c.Settings["baseUrl"].(string); ok && v != "" {
		return strings.TrimRight(v, "/")
	}
	if c.IsProduction() {
		return production
	}
	return sandbox
}

// MethodInfo is the part of a payment method a provider needs
type MethodInfo struct {
	Code    string
	Type    string
	Channel string
}

// ChargeRequest asks a gateway to create a payable object for one checkout attempt
type ChargeRequest struct {
	TransactionID string // used as order_id / external_id, echoed back in the callback
	Amount        int64  // total the customer pays, fee included
	Currency      string
	Method        MethodInfo
	Customer      shareddomain.Customer
	Items         []shareddomain.Item // fee is already a line item when > 0
	Description   string
	ExpiresAt     time.Time
	SuccessURL    string
	FailureURL    string
}

// ChargeResult is what the checkout page shows the customer
type ChargeResult struct {
	// Instruction is a flat, gateway independent map: type = virtual_account|qris|ewallet|redirect|...
	Instruction map[string]any
	// ExternalRef is the gateway's own id, needed to cancel
	ExternalRef string
	Raw         json.RawMessage
}

// CallbackResult is a verified, normalised gateway notification
type CallbackResult struct {
	TransactionID string
	Status        string // Callback* constants
	Amount        int64
	Reason        string
	PaidAt        time.Time
}

// Provider is implemented once per gateway
type Provider interface {
	Code() string
	CreateCharge(ctx context.Context, cfg Config, req ChargeRequest) (ChargeResult, error)
	// Cancel is best effort; callers log and ignore errors
	Cancel(ctx context.Context, cfg Config, transactionID, externalRef string) error
	// ParseCallback verifies authenticity (signature/token) and normalises the payload.
	// headers must have lower-case keys.
	ParseCallback(ctx context.Context, cfg Config, headers map[string]string, body []byte) (CallbackResult, error)
}

var httpClient = &http.Client{Timeout: 20 * time.Second}

// New returns the provider implementation for a gateway code
func New(code string) (Provider, error) {
	switch code {
	case shareddomain.GatewayMidtrans:
		return &Midtrans{client: httpClient}, nil
	case shareddomain.GatewayXendit:
		return &Xendit{client: httpClient}, nil
	case shareddomain.GatewayMock:
		return &Mock{}, nil
	}
	return nil, fmt.Errorf("unsupported gateway %q", code)
}

// Resolve checks a gateway row is usable and returns its provider and decrypted config
func Resolve(row shareddomain.Gateway, encryptionSecret string) (Provider, Config, error) {
	if !row.IsEnabled {
		return nil, Config{}, fmt.Errorf("gateway %s is disabled", row.Code)
	}
	cfg, err := BuildConfig(row, encryptionSecret)
	if err != nil {
		return nil, Config{}, err
	}
	if len(cfg.Credentials) == 0 && row.Code != shareddomain.GatewayMock {
		return nil, Config{}, fmt.Errorf("gateway %s has no credentials", row.Code)
	}
	p, err := New(row.Code)
	return p, cfg, err
}

// BuildConfig decrypts a gateway row's credentials; it does not check IsEnabled
func BuildConfig(row shareddomain.Gateway, encryptionSecret string) (Config, error) {
	cfg := Config{Code: row.Code, Environment: row.Environment, Settings: map[string]any{}, Credentials: map[string]string{}}
	if err := row.Settings.Decode(&cfg.Settings); err != nil {
		return cfg, fmt.Errorf("gateway %s settings: %w", row.Code, err)
	}
	if row.CredentialsEnc != "" {
		plain, err := crypto.Decrypt(encryptionSecret, row.CredentialsEnc)
		if err != nil {
			return cfg, fmt.Errorf("gateway %s credentials cannot be decrypted (was GATEWAY_ENCRYPTION_SECRET changed?)", row.Code)
		}
		if err := json.Unmarshal(plain, &cfg.Credentials); err != nil {
			return cfg, fmt.Errorf("gateway %s credentials: %w", row.Code, err)
		}
	}
	return cfg, nil
}

// doJSON sends a JSON request and decodes the JSON response into out (may be nil).
// A non-2xx status is returned as *HTTPError together with the raw body.
func doJSON(ctx context.Context, client *http.Client, method, url string, basicUser string, body any, out any) ([]byte, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = strings.NewReader(string(b))
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if basicUser != "" {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(basicUser+":")))
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return raw, &HTTPError{Status: res.StatusCode, Body: string(raw)}
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return raw, fmt.Errorf("decode gateway response: %w", err)
		}
	}
	return raw, nil
}

// HTTPError is a non-2xx response from a gateway
type HTTPError struct {
	Status int
	Body   string
}

func (e *HTTPError) Error() string {
	b := e.Body
	if len(b) > 300 {
		b = b[:300]
	}
	return fmt.Sprintf("gateway returned HTTP %d: %s", e.Status, b)
}

// reconcileItems makes sure the item lines add up to the gross amount, as Midtrans requires
func reconcileItems(req ChargeRequest) []shareddomain.Item {
	var sum int64
	for _, it := range req.Items {
		sum += it.Price * int64(it.Quantity)
	}
	if len(req.Items) > 0 && sum == req.Amount {
		return req.Items
	}
	name := req.Description
	if name == "" {
		name = "Payment"
	}
	return []shareddomain.Item{{Name: name, Price: req.Amount, Quantity: 1}}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
