package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/golangid/candi/candiutils"
)

type activityRESTImpl struct {
	host    string
	authKey string
	httpReq candiutils.HTTPRequest
}

// NewActivityServiceREST constructor
func NewActivityServiceREST(host string, authKey string) Activity {

	return &activityRESTImpl{
		host:    host,
		authKey: authKey,
		httpReq: candiutils.NewHTTPRequest(
			candiutils.HTTPRequestSetRetries(5),
			candiutils.HTTPRequestSetSleepBetweenRetry(500*time.Millisecond),
			candiutils.HTTPRequestSetHTTPErrorCodeThreshold(http.StatusBadRequest),
			candiutils.HTTPRequestSetBreakerName("activity"),
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

func (r *activityRESTImpl) headers() map[string]string {
	h := map[string]string{"Content-Type": "application/json"}
	if r.authKey != "" {
		h["Authorization"] = "Bearer " + r.authKey
	}
	return h
}

// SaveActivityLog calls POST /v1/activity — returns as soon as the server
// enqueues the entry, without waiting for it to be persisted.
func (r *activityRESTImpl) SaveActivityLog(ctx context.Context, req SaveActivityLogRequest) (jobID string, err error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	respBody, _, err := r.httpReq.Do(ctx, http.MethodPost, r.host+"/v1/activity", body, r.headers())
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

// GetAllActivityLogs calls GET /v1/activity
func (r *activityRESTImpl) GetAllActivityLogs(ctx context.Context, filter GetActivityLogsFilter) (result ActivityLogListResponse, err error) {
	query := url.Values{}
	if filter.Page > 0 {
		query.Set("page", fmt.Sprintf("%d", filter.Page))
	}
	if filter.Limit > 0 {
		query.Set("limit", fmt.Sprintf("%d", filter.Limit))
	}
	if filter.Search != "" {
		query.Set("search", filter.Search)
	}
	if filter.ServiceName != "" {
		query.Set("serviceName", filter.ServiceName)
	}
	if filter.ReferenceID != "" {
		query.Set("referenceId", filter.ReferenceID)
	}
	if filter.StartDate != "" {
		query.Set("startDate", filter.StartDate)
	}
	if filter.EndDate != "" {
		query.Set("endDate", filter.EndDate)
	}
	if filter.OrderBy != "" {
		query.Set("orderBy", filter.OrderBy)
	}
	if filter.Sort != "" {
		query.Set("sort", filter.Sort)
	}

	respBody, _, err := r.httpReq.Do(ctx, http.MethodGet, r.host+"/v1/activity?"+query.Encode(), nil, r.headers())
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

// GetActivityLogByID calls GET /v1/activity/{id}
func (r *activityRESTImpl) GetActivityLogByID(ctx context.Context, id string) (result ActivityLogResponse, err error) {
	respBody, _, err := r.httpReq.Do(ctx, http.MethodGet, r.host+"/v1/activity/"+url.PathEscape(id), nil, r.headers())
	if err != nil {
		return result, err
	}

	var envelope httpEnvelope
	if err = json.Unmarshal(respBody, &envelope); err != nil {
		return result, err
	}
	if len(envelope.Data) > 0 {
		err = json.Unmarshal(envelope.Data, &result)
	}
	return result, err
}
