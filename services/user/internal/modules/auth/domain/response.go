package domain

import (
	"time"

	menudomain "monorepo/services/user/internal/modules/menu/domain"
	shareddomain "monorepo/services/user/pkg/shared/domain"

	"github.com/golangid/candi/candishared"
)

// ResponseToken is the result of a successful login / refresh
type ResponseToken struct {
	AccessToken      string `json:"accessToken"`
	TokenType        string `json:"tokenType"`
	ExpiresIn        int    `json:"expiresIn"`
	RefreshToken     string `json:"refreshToken,omitempty"`
	RefreshExpiresIn int    `json:"refreshExpiresIn,omitempty"`
}

// ResponseSessionList model
type ResponseSessionList struct {
	Meta candishared.Meta  `json:"meta"`
	Data []ResponseSession `json:"data"`
}

// ResponseSession model
type ResponseSession struct {
	ID        int    `json:"id"`
	UserID    int    `json:"userId"`
	ClientID  int    `json:"clientId"`
	FamilyID  int    `json:"familyId"`
	IP        string `json:"ip"`
	UserAgent string `json:"userAgent"`
	Active    bool   `json:"active"`
	ExpiresAt string `json:"expiresAt"`
	CreatedAt string `json:"createdAt"`
}

// Serialize from db model
func (r *ResponseSession) Serialize(s *shareddomain.Session) {
	r.ID, r.UserID, r.ClientID, r.FamilyID, r.IP, r.UserAgent = s.ID, s.UserID, s.ClientID, s.FamilyID, s.IP, s.UserAgent
	r.Active = s.RevokedAt == nil && s.RotatedAt == nil && s.ExpiresAt.After(time.Now())
	r.ExpiresAt, r.CreatedAt = s.ExpiresAt.Format(time.RFC3339), s.CreatedAt.Format(time.RFC3339)
}

// ResponseMe is the profile of the authenticated principal
type ResponseMe struct {
	ID               int      `json:"id"`
	Realm            string   `json:"realm"`
	Username         string   `json:"username"`
	Email            string   `json:"email"`
	Phone            string   `json:"phone"`
	FullName         string   `json:"fullName"`
	IsServiceAccount bool     `json:"isServiceAccount"`
	Roles            []string `json:"roles"`
}

// ResponsePermissionRef is one granted permission
type ResponsePermissionRef struct {
	Service string `json:"service"`
	Code    string `json:"code"`
	Type    string `json:"type"`
}

// ResponseMyPermissions is what a frontend needs to render itself: the granted permission codes
// (to show / hide buttons) and the menu tree of the client filtered to what the user may open
type ResponseMyPermissions struct {
	Realm       string                    `json:"realm"`
	Roles       []string                  `json:"roles"`
	Permissions []ResponsePermissionRef   `json:"permissions"`
	Menus       []menudomain.ResponseMenu `json:"menus"`
}
