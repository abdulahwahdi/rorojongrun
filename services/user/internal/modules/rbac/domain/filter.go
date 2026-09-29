package domain

import "github.com/golangid/candi/candishared"

// FilterRole model
type FilterRole struct {
	candishared.Filter
	Name string `json:"name"`
}

// FilterPermission model
type FilterPermission struct {
	candishared.Filter
	Service string `json:"service"`
	Code    string `json:"code"`
	Type    string `json:"type"`
}
