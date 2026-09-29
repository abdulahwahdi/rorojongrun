package domain

import "github.com/golangid/candi/candishared"

// FilterClient model
type FilterClient struct {
	candishared.Filter
	ClientID string `json:"clientId"`
	Type     string `json:"type"`
	Enabled  *bool  `json:"enabled"`
}
