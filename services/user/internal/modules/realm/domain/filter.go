package domain

import "github.com/golangid/candi/candishared"

// FilterRealm model
type FilterRealm struct {
	candishared.Filter
	Name    string `json:"name"`
	Enabled *bool  `json:"enabled"`
}

// FilterRealmKey model
type FilterRealmKey struct {
	candishared.Filter
	Active *bool `json:"active"`
}
