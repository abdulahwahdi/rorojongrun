package domain

import (
	shareddomain "monorepo/services/activity/pkg/shared/domain"
	"time"

	"github.com/golangid/candi/candishared"
)

// ResponseActivityList model
type ResponseActivityList struct {
	Meta candishared.Meta   `json:"meta"`
	Data []ResponseActivity `json:"data"`
}

// ResponseActivity model
type ResponseActivity struct {
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

// Serialize from db model
func (r *ResponseActivity) Serialize(source *shareddomain.Activity) {
	r.ID = source.ID.Hex()
	r.ServiceName = source.ServiceName
	r.EventType = source.EventType
	r.ReferenceID = source.ReferenceID
	r.ActorID = source.ActorID
	r.Message = source.Message
	r.Metadata = source.Metadata
	r.CreatedAt = source.CreatedAt.Format(time.RFC3339)
	r.UpdatedAt = source.UpdatedAt.Format(time.RFC3339)
}

// ResponseSaveActivity model — returned immediately by the save endpoint
type ResponseSaveActivity struct {
	JobID string `json:"jobId"`
}
