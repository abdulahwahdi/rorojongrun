package domain

import (
	"time"

	shareddomain "monorepo/services/user/pkg/shared/domain"

	"github.com/golangid/candi/candishared"
)

// ResponseMenuList model
type ResponseMenuList struct {
	Meta candishared.Meta `json:"meta"`
	Data []ResponseMenu   `json:"data"`
}

// ResponseMenu model, Children is only filled in tree mode
type ResponseMenu struct {
	ID           int            `json:"id"`
	Key          string         `json:"key"`
	Label        string         `json:"label"`
	Path         string         `json:"path"`
	Icon         string         `json:"icon"`
	SortOrder    int            `json:"sortOrder"`
	ParentID     int            `json:"parentId"`
	PermissionID int            `json:"permissionId"`
	Permission   string         `json:"permission"` // "<service>:<code>" of the linked permission
	Children     []ResponseMenu `json:"children"`
	CreatedAt    string         `json:"createdAt"`
	UpdatedAt    string         `json:"updatedAt"`
}

// Serialize from db model
func (r *ResponseMenu) Serialize(s *shareddomain.Menu, perms map[int]shareddomain.Permission) {
	r.ID, r.Key, r.Label, r.Path, r.Icon, r.SortOrder = s.ID, s.Key, s.Label, s.Path, s.Icon, s.SortOrder
	if s.ParentID != nil {
		r.ParentID = *s.ParentID
	}
	if s.PermissionID != nil {
		r.PermissionID = *s.PermissionID
		if p, ok := perms[*s.PermissionID]; ok {
			r.Permission = p.Service + ":" + p.Code
		}
	}
	r.Children = []ResponseMenu{}
	r.CreatedAt, r.UpdatedAt = s.CreatedAt.Format(time.RFC3339), s.UpdatedAt.Format(time.RFC3339)
}
