package provider

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	shareddomain "monorepo/services/payment/pkg/shared/domain"
)

// Mock is a dev/test gateway without any network call. Credentials: callbackToken (optional).
// Callback body: {"transactionId":"...","status":"paid|failed|expired","amount":12345}
// with the token in the x-mock-token header when configured.
type Mock struct{}

// Code implements Provider
func (m *Mock) Code() string { return shareddomain.GatewayMock }

// CreateCharge implements Provider
func (m *Mock) CreateCharge(_ context.Context, cfg Config, req ChargeRequest) (ChargeResult, error) {
	if cfg.IsProduction() {
		return ChargeResult{}, errors.New("mock gateway cannot be used in production")
	}
	sum := sha256.Sum256([]byte(req.TransactionID))
	va := fmt.Sprintf("88%010d", binary.BigEndian.Uint64(sum[:8])%10000000000)
	return ChargeResult{
		Instruction: map[string]any{"type": shareddomain.MethodVirtualAccount, "bank": "MOCK", "vaNumber": va, "channel": req.Method.Channel},
		ExternalRef: "mock-" + req.TransactionID,
	}, nil
}

// Cancel implements Provider
func (m *Mock) Cancel(context.Context, Config, string, string) error { return nil }

// ParseCallback implements Provider
func (m *Mock) ParseCallback(_ context.Context, cfg Config, headers map[string]string, body []byte) (CallbackResult, error) {
	if cfg.IsProduction() {
		return CallbackResult{}, fmt.Errorf("%w: mock gateway cannot be used in production", ErrInvalidCallback)
	}
	if token := cfg.Credentials["callbackToken"]; token != "" &&
		subtle.ConstantTimeCompare([]byte(headers["x-mock-token"]), []byte(token)) != 1 {
		return CallbackResult{}, fmt.Errorf("%w: token mismatch", ErrInvalidCallback)
	}
	var n struct {
		TransactionID string `json:"transactionId"`
		Status        string `json:"status"`
		Amount        int64  `json:"amount"`
	}
	if err := json.Unmarshal(body, &n); err != nil || n.TransactionID == "" {
		return CallbackResult{}, fmt.Errorf("%w: unreadable body", ErrInvalidCallback)
	}
	res := CallbackResult{TransactionID: n.TransactionID, Amount: n.Amount}
	switch n.Status {
	case "paid":
		res.Status, res.PaidAt = CallbackPaid, time.Now()
	case "failed":
		res.Status, res.Reason = CallbackFailed, "mock: failed"
	case "expired":
		res.Status = CallbackExpired
	default:
		res.Status = CallbackIgnored
	}
	return res, nil
}
