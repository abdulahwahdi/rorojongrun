package domain

import "github.com/golangid/candi/candishared"

// FilterMethod model
type FilterMethod struct {
	candishared.Filter
	Type        string `json:"type"`
	GatewayCode string `json:"gatewayCode"`
	IsEnabled   *bool  `json:"isEnabled"`
}
