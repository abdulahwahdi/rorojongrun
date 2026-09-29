package domain

import "github.com/golangid/candi/candishared"

// FilterMenu model
type FilterMenu struct {
	candishared.Filter
	ParentID *int `json:"parentId"`
	// Tree returns the whole tree nested (paging is ignored) instead of a flat page
	Tree bool `json:"tree"`
}
