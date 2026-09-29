package domain

import (
	"strings"
	"time"

	shareddomain "monorepo/services/user/pkg/shared/domain"

	"github.com/golangid/candi/candishared"
)

// ResponseClientList model
type ResponseClientList struct {
	Meta candishared.Meta `json:"meta"`
	Data []ResponseClient `json:"data"`
}

// ResponseClient model. ClientSecret is only filled when it was just generated (create / rotate)
// and can never be retrieved again.
type ResponseClient struct {
	ID            int      `json:"id"`
	ClientID      string   `json:"clientId"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Type          string   `json:"type"`
	GrantTypes    []string `json:"grantTypes"`
	Enabled       bool     `json:"enabled"`
	ServiceUserID int      `json:"serviceUserId"`
	ClientSecret  string   `json:"clientSecret,omitempty"`
	CreatedAt     string   `json:"createdAt"`
	UpdatedAt     string   `json:"updatedAt"`
}

// Serialize from db model
func (r *ResponseClient) Serialize(s *shareddomain.Client) {
	r.ID, r.ClientID, r.Name, r.Description, r.Type, r.Enabled = s.ID, s.ClientID, s.Name, s.Description, s.Type, s.Enabled
	r.GrantTypes = []string{}
	if s.GrantTypes != "" {
		r.GrantTypes = strings.Split(s.GrantTypes, ",")
	}
	if s.ServiceUserID != nil {
		r.ServiceUserID = *s.ServiceUserID
	}
	r.CreatedAt, r.UpdatedAt = s.CreatedAt.Format(time.RFC3339), s.UpdatedAt.Format(time.RFC3339)
}
