package domain

import (
	"sort"
	"time"

	shareddomain "monorepo/services/payment/pkg/shared/domain"
)

// ResponseGateway never carries credentials, only which keys are set (masked)
type ResponseGateway struct {
	ID             int               `json:"id"`
	Code           string            `json:"code"`
	Name           string            `json:"name"`
	IsEnabled      bool              `json:"isEnabled"`
	Environment    string            `json:"environment"`
	Settings       map[string]any    `json:"settings"`
	HasCredentials bool              `json:"hasCredentials"`
	Credentials    map[string]string `json:"credentials"`
	CreatedAt      string            `json:"createdAt"`
	UpdatedAt      string            `json:"updatedAt"`
}

// Serialize from db model; creds are the decrypted credentials, exposed masked only
func (r *ResponseGateway) Serialize(src *shareddomain.Gateway, creds map[string]string) {
	r.ID, r.Code, r.Name, r.IsEnabled, r.Environment = src.ID, src.Code, src.Name, src.IsEnabled, src.Environment
	r.Settings = map[string]any{}
	_ = src.Settings.Decode(&r.Settings)
	r.HasCredentials = len(creds) > 0
	r.Credentials = map[string]string{}
	for k, v := range creds {
		r.Credentials[k] = Mask(v)
	}
	r.CreatedAt = src.CreatedAt.Format(time.RFC3339)
	r.UpdatedAt = src.UpdatedAt.Format(time.RFC3339)
}

// Mask hides all but the last 4 characters of a secret
func Mask(v string) string {
	if len(v) <= 8 {
		return "****"
	}
	return "****" + v[len(v)-4:]
}

// CredentialKeys lists the keys of creds sorted, for logging without values
func CredentialKeys(creds map[string]string) []string {
	keys := make([]string, 0, len(creds))
	for k := range creds {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
