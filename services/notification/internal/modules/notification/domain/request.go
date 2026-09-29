package domain

// RequestSendNotification is the public payload for triggering a
// notification, shared by the REST send endpoint, the Kafka handler, and
// the otp module's cross-module call.
type RequestSendNotification struct {
	Channel      string         `json:"channel"`
	TemplateCode string         `json:"templateCode"`
	Recipient    string         `json:"recipient"`
	Variables    map[string]any `json:"variables,omitempty"`
}

// RequestUpsertTemplate is the payload for creating/updating a template.
type RequestUpsertTemplate struct {
	Code            string `json:"code"`
	Channel         string `json:"channel"`
	SubjectTemplate string `json:"subjectTemplate,omitempty"`
	BodyTemplate    string `json:"bodyTemplate"`
	Variables       string `json:"variables,omitempty"`
	IsActive        bool   `json:"isActive"`
}

// RequestUpsertChannelConfig is the payload for updating a channel's config.
type RequestUpsertChannelConfig struct {
	IsEnabled bool           `json:"isEnabled"`
	Settings  map[string]any `json:"settings,omitempty"`
}

// DispatchPayload is the internal (non-REST) task-queue job payload — the
// notification has already been rendered by the time this is enqueued, so
// the worker doesn't need to re-look-up the template/config.
type DispatchPayload struct {
	LogID           int    `json:"logId"`
	Channel         string `json:"channel"`
	TemplateCode    string `json:"templateCode"`
	Recipient       string `json:"recipient"`
	RenderedSubject string `json:"renderedSubject,omitempty"`
	RenderedBody    string `json:"renderedBody"`
}
