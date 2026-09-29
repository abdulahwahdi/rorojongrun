package notification

import (
	"context"
	"encoding/json"
	"fmt"
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
