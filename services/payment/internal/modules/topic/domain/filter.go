package domain

import "github.com/golangid/candi/candishared"

// FilterTopic model
type FilterTopic struct {
	candishared.Filter
	Direction   string `json:"direction"`
	GatewayCode string `json:"gatewayCode"`
	IsEnabled   *bool  `json:"isEnabled"`
}
