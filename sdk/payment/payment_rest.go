package payment

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/golangid/candi/candiutils"
)

type paymentRESTImpl struct {
	host    string
	authKey string
	httpReq candiutils.HTTPRequest
}

// NewPaymentServiceREST constructor
func NewPaymentServiceREST(host string, authKey string) Payment {

	return &paymentRESTImpl{
		host:    host,
		authKey: authKey,
		httpReq: candiutils.NewHTTPRequest(
			candiutils.HTTPRequestSetRetries(5),
			candiutils.HTTPRequestSetSleepBetweenRetry(500*time.Millisecond),
			candiutils.HTTPRequestSetHTTPErrorCodeThreshold(http.StatusBadRequest),
			candiutils.HTTPRequestSetBreakerName("payment"),
		),
	}
}

// httpEnvelope mirrors candi's wrapper.HTTPResponse JSON shape.
type httpEnvelope struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (r *paymentRESTImpl) headers() map[string]string {
	h := map[string]string{"Content-Type": "application/json"}
	if r.authKey != "" {
		h["Authorization"] = "Bearer " + r.authKey
	}
	return h
}

// call sends a request and decodes the envelope's data into out
func (r *paymentRESTImpl) call(ctx context.Context, method, path string, in, out any) error {
	var body []byte
	if in != nil {
		var err error
		if body, err = json.Marshal(in); err != nil {
			return err
		}
	}
	respBody, _, err := r.httpReq.Do(ctx, method, r.host+path, body, r.headers())
	if err != nil {
		return err
	}
	var envelope httpEnvelope
	if err = json.Unmarshal(respBody, &envelope); err != nil {
		return err
	}
	return json.Unmarshal(envelope.Data, out)
}

// CreatePayment calls POST /v1/payments. authKey is a bearer token of a service client.
func (r *paymentRESTImpl) CreatePayment(ctx context.Context, req CreatePaymentRequest) (res CreatePaymentResponse, err error) {
	err = r.call(ctx, http.MethodPost, "/v1/payments", req, &res)
	return
}

// GetPayment calls GET /v1/payments/{id}
func (r *paymentRESTImpl) GetPayment(ctx context.Context, id string) (res PaymentResponse, err error) {
	err = r.call(ctx, http.MethodGet, "/v1/payments/"+id, nil, &res)
	return
}

// CancelPayment calls POST /v1/payments/{id}/cancel
func (r *paymentRESTImpl) CancelPayment(ctx context.Context, id string) (res PaymentResponse, err error) {
	err = r.call(ctx, http.MethodPost, "/v1/payments/"+id+"/cancel", nil, &res)
	return
}
