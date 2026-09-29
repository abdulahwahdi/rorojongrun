package domain

import "github.com/golangid/candi/candishared"

// FilterActivity model
type FilterActivity struct {
	candishared.Filter
	ID          *string `json:"id"`
	ServiceName string  `json:"serviceName"`
	ReferenceID string  `json:"referenceId"`
	StartDate   string  `json:"startDate"`
	EndDate     string  `json:"endDate"`
}
