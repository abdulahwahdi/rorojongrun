package activity

import "context"

// Activity client abstract interface
type Activity interface {
	// SaveActivityLog saves an activity/audit log entry. The call returns as
	// soon as the entry is enqueued server-side — it does not wait for the
	// entry to actually be persisted.
	SaveActivityLog(ctx context.Context, req SaveActivityLogRequest) (jobID string, err error)
	GetAllActivityLogs(ctx context.Context, filter GetActivityLogsFilter) (ActivityLogListResponse, error)
	GetActivityLogByID(ctx context.Context, id string) (ActivityLogResponse, error)
}

// SaveActivityLogRequest is the payload for SaveActivityLog. Kept as a local,
// intentionally-duplicated copy of services/activity's request shape — sdk/
// clients must not import a service's internal/ packages.
type SaveActivityLogRequest struct {
	ServiceName string         `json:"serviceName"`
	EventType   string         `json:"eventType"`
	ReferenceID string         `json:"referenceId"`
	ActorID     string         `json:"actorId,omitempty"`
	Message     string         `json:"message,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// GetActivityLogsFilter is the query filter for GetAllActivityLogs.
type GetActivityLogsFilter struct {
	Page        int    `json:"page,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	Search      string `json:"search,omitempty"`
	ServiceName string `json:"serviceName,omitempty"`
	ReferenceID string `json:"referenceId,omitempty"`
	StartDate   string `json:"startDate,omitempty"`
	EndDate     string `json:"endDate,omitempty"`
	OrderBy     string `json:"orderBy,omitempty"`
	Sort        string `json:"sort,omitempty"`
}

// ActivityLogResponse mirrors services/activity's domain.ResponseActivity JSON shape.
type ActivityLogResponse struct {
	ID          string         `json:"id"`
	ServiceName string         `json:"serviceName"`
	EventType   string         `json:"eventType"`
	ReferenceID string         `json:"referenceId"`
	ActorID     string         `json:"actorId,omitempty"`
	Message     string         `json:"message,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedAt   string         `json:"createdAt"`
	UpdatedAt   string         `json:"updatedAt"`
}

// ActivityLogListResponse mirrors services/activity's domain.ResponseActivityList JSON shape.
type ActivityLogListResponse struct {
	Meta struct {
		Page         int `json:"page"`
		Limit        int `json:"limit"`
		TotalRecords int `json:"totalRecords"`
		TotalPages   int `json:"totalPages"`
	} `json:"meta"`
	Data []ActivityLogResponse `json:"data"`
}
