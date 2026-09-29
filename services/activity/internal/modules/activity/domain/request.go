package domain

import (
	shareddomain "monorepo/services/activity/pkg/shared/domain"
)

// RequestSaveActivity model — public payload for saving an activity log entry
type RequestSaveActivity struct {
	ServiceName string         `json:"serviceName"`
	EventType   string         `json:"eventType"`
	ReferenceID string         `json:"referenceId"`
	ActorID     string         `json:"actorId,omitempty"`
	Message     string         `json:"message,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// Deserialize to db model
func (r *RequestSaveActivity) Deserialize() (res shareddomain.Activity) {
	res.ServiceName = r.ServiceName
	res.EventType = r.EventType
	res.ReferenceID = r.ReferenceID
	res.ActorID = r.ActorID
	res.Message = r.Message
	res.Metadata = r.Metadata
	return
}
