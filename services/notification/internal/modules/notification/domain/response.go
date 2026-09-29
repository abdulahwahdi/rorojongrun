package domain

import (
	"encoding/json"
	"time"

	shareddomain "monorepo/services/notification/pkg/shared/domain"

	"github.com/golangid/candi/candishared"
)

// ResponseSendNotification model — returned immediately by the send endpoint
type ResponseSendNotification struct {
	JobID string `json:"jobId"`
}

// ResponseTemplateList model
type ResponseTemplateList struct {
	Meta candishared.Meta   `json:"meta"`
	Data []ResponseTemplate `json:"data"`
}

// ResponseTemplate model
type ResponseTemplate struct {
	ID              int    `json:"id"`
	Code            string `json:"code"`
	Channel         string `json:"channel"`
	SubjectTemplate string `json:"subjectTemplate,omitempty"`
	BodyTemplate    string `json:"bodyTemplate"`
	Variables       string `json:"variables,omitempty"`
	IsActive        bool   `json:"isActive"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

// Serialize from db model
func (r *ResponseTemplate) Serialize(source *shareddomain.NotificationTemplate) {
	r.ID = source.ID
	r.Code = source.Code
	r.Channel = source.Channel
	if source.SubjectTemplate != nil {
		r.SubjectTemplate = *source.SubjectTemplate
	}
	r.BodyTemplate = source.BodyTemplate
	if source.Variables != nil {
		r.Variables = *source.Variables
	}
	r.IsActive = source.IsActive
	r.CreatedAt = source.CreatedAt.Format(time.RFC3339)
	r.UpdatedAt = source.UpdatedAt.Format(time.RFC3339)
}

// ResponseChannelConfig model
type ResponseChannelConfig struct {
	ID        int            `json:"id"`
	Channel   string         `json:"channel"`
	IsEnabled bool           `json:"isEnabled"`
	Settings  map[string]any `json:"settings,omitempty"`
	CreatedAt string         `json:"createdAt"`
	UpdatedAt string         `json:"updatedAt"`
}

// Serialize from db model
func (r *ResponseChannelConfig) Serialize(source *shareddomain.NotificationChannelConfig) {
	r.ID = source.ID
	r.Channel = source.Channel
	r.IsEnabled = source.IsEnabled
	if len(source.Settings) > 0 {
		_ = json.Unmarshal(source.Settings, &r.Settings)
	}
	r.CreatedAt = source.CreatedAt.Format(time.RFC3339)
	r.UpdatedAt = source.UpdatedAt.Format(time.RFC3339)
}

// ResponseNotificationLogList model
type ResponseNotificationLogList struct {
	Meta candishared.Meta        `json:"meta"`
	Data []ResponseNotificationLog `json:"data"`
}

// ResponseNotificationLog model
type ResponseNotificationLog struct {
	ID              int    `json:"id"`
	Channel         string `json:"channel"`
	TemplateCode    string `json:"templateCode"`
	Recipient       string `json:"recipient"`
	RenderedSubject string `json:"renderedSubject,omitempty"`
	RenderedBody    string `json:"renderedBody,omitempty"`
	Status          string `json:"status"`
	ErrorMessage    string `json:"errorMessage,omitempty"`
	SentAt          string `json:"sentAt,omitempty"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

// Serialize from db model
func (r *ResponseNotificationLog) Serialize(source *shareddomain.NotificationLog) {
	r.ID = source.ID
	r.Channel = source.Channel
	r.TemplateCode = source.TemplateCode
	r.Recipient = source.Recipient
	if source.RenderedSubject != nil {
		r.RenderedSubject = *source.RenderedSubject
	}
	if source.RenderedBody != nil {
		r.RenderedBody = *source.RenderedBody
	}
	r.Status = source.Status
	if source.ErrorMessage != nil {
		r.ErrorMessage = *source.ErrorMessage
	}
	if source.SentAt != nil {
		r.SentAt = source.SentAt.Format(time.RFC3339)
	}
	r.CreatedAt = source.CreatedAt.Format(time.RFC3339)
	r.UpdatedAt = source.UpdatedAt.Format(time.RFC3339)
}
