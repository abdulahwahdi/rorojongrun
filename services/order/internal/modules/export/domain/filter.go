package domain

import "github.com/golangid/candi/candishared"

// FilterExport model
type FilterExport struct {
	candishared.Filter
	Type   string `json:"type,omitempty"`
	Status string `json:"status,omitempty"`
	// RequestedBy limits the list to one requester; set by the usecase for callers without manageExports
	RequestedBy string `json:"-"`
}
