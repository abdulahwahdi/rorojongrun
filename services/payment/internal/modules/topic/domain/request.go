package domain

// RequestSaveTopic creates or replaces a Kafka topic config
type RequestSaveTopic struct {
	Topic       string `json:"topic"`
	Direction   string `json:"direction"`
	GatewayCode string `json:"gatewayCode"`
	EventType   string `json:"eventType"`
	IsEnabled   bool   `json:"isEnabled"`
}

// RequestSetStatus enables or disables a topic
type RequestSetStatus struct {
	IsEnabled bool `json:"isEnabled"`
}
