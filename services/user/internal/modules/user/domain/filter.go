package domain

import "github.com/golangid/candi/candishared"

// FilterUser model
type FilterUser struct {
	candishared.Filter
	Username string `json:"username"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Status   string `json:"status"`
	// Role filters by role name
	Role string `json:"role"`
	// IncludeServiceAccounts also lists the machine users behind confidential clients
	IncludeServiceAccounts bool `json:"includeServiceAccounts"`
}
