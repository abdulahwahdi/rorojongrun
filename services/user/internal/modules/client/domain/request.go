package domain

// RequestCreateClient is the payload to register a client
type RequestCreateClient struct {
	ClientID    string   `json:"clientId"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Type        string   `json:"type"` // public | confidential
	GrantTypes  []string `json:"grantTypes"`
	Enabled     *bool    `json:"enabled"`
}

// RequestUpdateClient is the payload to update a client (client id and type are immutable)
type RequestUpdateClient struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	GrantTypes  []string `json:"grantTypes"`
	Enabled     *bool    `json:"enabled"`
}
