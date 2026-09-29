package domain

import "github.com/golangid/candi/candishared"

// FilterSession model
type FilterSession struct {
	candishared.Filter
	UserID *int `json:"userId"`
	// ActiveOnly lists only live sessions: current refresh token, not revoked, not expired
	ActiveOnly bool `json:"activeOnly"`
}
