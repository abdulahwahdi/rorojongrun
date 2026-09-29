package domain

import "github.com/golangid/candi/candishared"

// FilterNotificationLog model
type FilterNotificationLog struct {
	candishared.Filter
	ID           *int   `json:"id"`
	Channel      string `json:"channel"`
	TemplateCode string `json:"templateCode"`
	Recipient    string `json:"recipient"`
	Status       string `json:"status"`
}

// FilterTemplate model
type FilterTemplate struct {
	candishared.Filter
	Channel string `json:"channel"`
}
