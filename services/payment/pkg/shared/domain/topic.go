package domain

import "time"

// Topic model — a Kafka topic managed from the DB. Consume topics carry
// gateway callbacks, publish topics carry outbound events.
type Topic struct {
	ID          int       `gorm:"column:id;primary_key" json:"id"`
	Topic       string    `gorm:"column:topic;type:varchar(200)" json:"topic"`
	Direction   string    `gorm:"column:direction;type:varchar(10)" json:"direction"`
	GatewayCode *string   `gorm:"column:gateway_code;type:varchar(50)" json:"gatewayCode"`
	EventType   *string   `gorm:"column:event_type;type:varchar(50)" json:"eventType"`
	IsEnabled   bool      `gorm:"column:is_enabled" json:"isEnabled"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName return table name of Topic model
func (Topic) TableName() string { return "payment_kafka_topics" }
