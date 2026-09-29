package domain

import (
	"time"

	shareddomain "monorepo/services/user/pkg/shared/domain"

	"github.com/golangid/candi/candishared"
)

// ResponseRoleList model
type ResponseRoleList struct {
	Meta candishared.Meta `json:"meta"`
	Data []ResponseRole   `json:"data"`
}

// ResponseRole model, Permissions is only filled on detail
type ResponseRole struct {
	ID          int                  `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Permissions []ResponsePermission `json:"permissions"`
	CreatedAt   string               `json:"createdAt"`
	UpdatedAt   string               `json:"updatedAt"`
}

// Serialize from db model
func (r *ResponseRole) Serialize(s *shareddomain.Role) {
	r.ID, r.Name, r.Description = s.ID, s.Name, s.Description
	r.CreatedAt, r.UpdatedAt = s.CreatedAt.Format(time.RFC3339), s.UpdatedAt.Format(time.RFC3339)
	r.Permissions = []ResponsePermission{}
}

// ResponsePermissionList model
type ResponsePermissionList struct {
	Meta candishared.Meta     `json:"meta"`
	Data []ResponsePermission `json:"data"`
}

// ResponsePermission model
type ResponsePermission struct {
	ID          int    `json:"id"`
	Service     string `json:"service"`
	Code        string `json:"code"`
	Type        string `json:"type"`
	Description string `json:"description"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

// Serialize from db model
func (r *ResponsePermission) Serialize(s *shareddomain.Permission) {
	r.ID, r.Service, r.Code, r.Type, r.Description = s.ID, s.Service, s.Code, s.Type, s.Description
	r.CreatedAt, r.UpdatedAt = s.CreatedAt.Format(time.RFC3339), s.UpdatedAt.Format(time.RFC3339)
}
