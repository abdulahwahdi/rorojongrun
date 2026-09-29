package domain

import (
	"time"

	shareddomain "monorepo/services/user/pkg/shared/domain"

	"github.com/golangid/candi/candishared"
)

// ResponseUserList model
type ResponseUserList struct {
	Meta candishared.Meta `json:"meta"`
	Data []ResponseUser   `json:"data"`
}

// ResponseRoleRef is a role a user holds
type ResponseRoleRef struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ResponseUser model, Roles is only filled on detail
type ResponseUser struct {
	ID               int               `json:"id"`
	Username         string            `json:"username"`
	Email            string            `json:"email"`
	Phone            string            `json:"phone"`
	FullName         string            `json:"fullName"`
	Status           string            `json:"status"`
	LockedUntil      string            `json:"lockedUntil"`
	LastLoginAt      string            `json:"lastLoginAt"`
	IsServiceAccount bool              `json:"isServiceAccount"`
	Roles            []ResponseRoleRef `json:"roles"`
	CreatedAt        string            `json:"createdAt"`
	UpdatedAt        string            `json:"updatedAt"`
}

// Serialize from db model
func (r *ResponseUser) Serialize(s *shareddomain.User) {
	r.ID, r.Username, r.FullName, r.Status, r.IsServiceAccount = s.ID, s.Username, s.FullName, s.Status, s.IsServiceAccount
	if s.Email != nil {
		r.Email = *s.Email
	}
	if s.Phone != nil {
		r.Phone = *s.Phone
	}
	if s.LockedUntil != nil && s.LockedUntil.After(time.Now()) {
		r.LockedUntil = s.LockedUntil.Format(time.RFC3339)
	}
	if s.LastLoginAt != nil {
		r.LastLoginAt = s.LastLoginAt.Format(time.RFC3339)
	}
	r.Roles = []ResponseRoleRef{}
	r.CreatedAt, r.UpdatedAt = s.CreatedAt.Format(time.RFC3339), s.UpdatedAt.Format(time.RFC3339)
}
