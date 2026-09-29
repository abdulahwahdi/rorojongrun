package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Activity model — an audit/activity log entry written by any service
type Activity struct {
	ID          bson.ObjectID  `bson:"_id" json:"id"`
	ServiceName string         `bson:"service_name" json:"serviceName"`
	EventType   string         `bson:"event_type" json:"eventType"`
	ReferenceID string         `bson:"reference_id" json:"referenceId"`
	ActorID     string         `bson:"actor_id,omitempty" json:"actorId,omitempty"`
	Message     string         `bson:"message,omitempty" json:"message,omitempty"`
	Metadata    map[string]any `bson:"metadata,omitempty" json:"metadata,omitempty"`
	CreatedAt   time.Time      `bson:"created_at" json:"createdAt"`
	UpdatedAt   time.Time      `bson:"updated_at" json:"updatedAt"`
}

// CollectionName return collection name of Activity model
func (Activity) CollectionName() string {
	return "activities"
}
