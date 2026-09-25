package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/golangid/candi/candiutils"
)

type notificationRESTImpl struct {
	host    string
	authKey string
	httpReq candiutils.HTTPRequest
}

// NewNotificationServiceREST constructor
func NewNotificationServiceREST(host string, authKey string) Notification {

	return &notificationRESTImpl{
		host:    host,
		authKey: authKey,
		httpReq: candiutils.NewHTTPRequest(
			candiutils.HTTPRequestSetRetries(5),
			candiutils.HTTPRequestSetSleepBetweenRetry(500*time.Millisecond),
			candiutils.HTTPRequestSetHTTPErrorCodeThreshold(http.StatusBadRequest),
			candiutils.HTTPRequestSetBreakerName("notification"),
		),
	}
}

// httpEnvelope mirrors candi's wrapper.HTTPResponse JSON shape.
type httpEnvelope struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Meta    json.RawMessage `json:"meta,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (r *notificationRESTImpl) headers() map[string]string {
	h := map[string]string{"Content-Type": "application/json"}
	if r.authKey != "" {
		h["Authorization"] = "Bearer " + r.authKey
	}
	return h
}

// SendNotification calls POST /v1/notification/send — returns as soon as
// the server enqueues the entry, without waiting for delivery.
func (r *notificationRESTImpl) SendNotification(ctx context.Context, req SendNotificationRequest) (jobID string, err error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	respBody, _, err := r.httpReq.Do(ctx, http.MethodPost, r.host+"/v1/notification/send", body, r.headers())
	if err != nil {
		return "", err
	}

	var envelope httpEnvelope
	if err = json.Unmarshal(respBody, &envelope); err != nil {
		return "", err
	}

	var data struct {
		JobID string `json:"jobId"`
	}
	if err = json.Unmarshal(envelope.Data, &data); err != nil {
		return "", err
	}
	return data.JobID, nil
}

// GetAllNotificationLogs calls GET /v1/notification/logs
func (r *notificationRESTImpl) GetAllNotificationLogs(ctx context.Context, filter GetNotificationLogsFilter) (result NotificationLogListResponse, err error) {
	query := url.Values{}
	if filter.Page > 0 {
		query.Set("page", fmt.Sprintf("%d", filter.Page))
	}
	if filter.Limit > 0 {
		query.Set("limit", fmt.Sprintf("%d", filter.Limit))
	}
	if filter.Channel != "" {
		query.Set("channel", filter.Channel)
	}
	if filter.TemplateCode != "" {
		query.Set("templateCode", filter.TemplateCode)
	}
	if filter.Recipient != "" {
		query.Set("recipient", filter.Recipient)
	}
	if filter.Status != "" {
		query.Set("status", filter.Status)
	}

	respBody, _, err := r.httpReq.Do(ctx, http.MethodGet, r.host+"/v1/notification/logs?"+query.Encode(), nil, r.headers())
	if err != nil {
		return result, err
	}

	var envelope httpEnvelope
	if err = json.Unmarshal(respBody, &envelope); err != nil {
		return result, err
	}
	if len(envelope.Meta) > 0 {
		if err = json.Unmarshal(envelope.Meta, &result.Meta); err != nil {
			return result, err
		}
	}
	if len(envelope.Data) > 0 {
		if err = json.Unmarshal(envelope.Data, &result.Data); err != nil {
			return result, err
		}
	}
	return result, nil
}

// RequestOTP calls POST /v1/otp/request
func (r *notificationRESTImpl) RequestOTP(ctx context.Context, req RequestOTPRequest) (jobID string, err error) {
	respBody, status, err := r.doOnce(ctx, http.MethodPost, "/v1/otp/request", req)
	if err != nil {
		return "", err
	}
	if status != http.StatusAccepted && status != http.StatusOK {
		return "", fmt.Errorf("notification: request otp failed with status %d: %s", status, string(respBody))
	}
	var envelope httpEnvelope
	if err = json.Unmarshal(respBody, &envelope); err != nil {
		return "", err
	}
	var data struct {
		JobID string `json:"jobId"`
	}
	if err = json.Unmarshal(envelope.Data, &data); err != nil {
		return "", err
	}
	return data.JobID, nil
}

// VerifyOTP calls POST /v1/otp/verify. Rejections (400 mismatch, 404 unknown, 410 expired,
// 429 too many attempts) are verified=false with a nil error. It is never retried: every
// attempt counts against the code's attempt limit on the server.
func (r *notificationRESTImpl) VerifyOTP(ctx context.Context, req VerifyOTPRequest) (verified bool, err error) {
	respBody, status, err := r.doOnce(ctx, http.MethodPost, "/v1/otp/verify", req)
	if err != nil {
		return false, err
	}
	switch status {
	case http.StatusOK:
		return true, nil
	case http.StatusBadRequest, http.StatusNotFound, http.StatusGone, http.StatusTooManyRequests:
		return false, nil
	}
	return false, fmt.Errorf("notification: verify otp failed with status %d: %s", status, string(respBody))
}

// doOnce performs a single JSON request without the retrying client, returning body and status.
func (r *notificationRESTImpl) doOnce(ctx context.Context, method, path string, payload any) ([]byte, int, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, method, r.host+path, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	for k, v := range r.headers() {
		httpReq.Header.Set(k, v)
	}
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(httpReq)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	return respBody, resp.StatusCode, err
}
