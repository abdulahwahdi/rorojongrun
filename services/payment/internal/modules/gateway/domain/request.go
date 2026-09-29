package domain

// RequestUpdateGateway updates a gateway's settings and credentials. Credentials are
// write-only: they are merged into the stored ones, an empty value deletes that key.
type RequestUpdateGateway struct {
	Name        string            `json:"name"`
	Environment string            `json:"environment"`
	Settings    map[string]any    `json:"settings"`
	Credentials map[string]string `json:"credentials"`
}

// RequestSetStatus enables or disables a gateway
type RequestSetStatus struct {
	IsEnabled bool `json:"isEnabled"`
}
