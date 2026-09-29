package domain

// RequestSaveMethod creates or replaces a payment method
type RequestSaveMethod struct {
	Code           string  `json:"code"`
	Name           string  `json:"name"`
	Type           string  `json:"type"`
	GatewayCode    string  `json:"gatewayCode"`
	GatewayChannel string  `json:"gatewayChannel"`
	IsEnabled      bool    `json:"isEnabled"`
	IconURL        string  `json:"iconUrl"`
	SortOrder      int     `json:"sortOrder"`
	MinAmount      int64   `json:"minAmount"`
	MaxAmount      int64   `json:"maxAmount"`
	FeeFlat        int64   `json:"feeFlat"`
	FeePercent     float64 `json:"feePercent"`
	Instructions   string  `json:"instructions"`
}

// RequestSetStatus enables or disables a method
type RequestSetStatus struct {
	IsEnabled bool `json:"isEnabled"`
}
