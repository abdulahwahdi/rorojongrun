package domain

import "github.com/golangid/candi/candishared"

// FilterPayment model
type FilterPayment struct {
	candishared.Filter
	Source      string `json:"source"`
	ReferenceID string `json:"referenceId"`
	Status      string `json:"status"`
	MethodCode  string `json:"methodCode"`
	StartDate   string `json:"startDate"`
	EndDate     string `json:"endDate"`
}

// FilterCallbackLog model
type FilterCallbackLog struct {
	candishared.Filter
	GatewayCode string `json:"gatewayCode"`
	Status      string `json:"status"`
	ExternalID  string `json:"externalId"`
	Topic       string `json:"topic"`
}
