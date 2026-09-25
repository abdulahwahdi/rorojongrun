package notification

import "context"

// Notification client abstract interface
type Notification interface {
	// SendNotification triggers a notification send. The call returns as
	// soon as the entry is enqueued server-side — it does not wait for the
	// notification to actually be delivered.
	SendNotification(ctx context.Context, req SendNotificationRequest) (jobID string, err error)
	GetAllNotificationLogs(ctx context.Context, filter GetNotificationLogsFilter) (NotificationLogListResponse, error)
	// RequestOTP generates an OTP code and delivers it; returns as soon as delivery is enqueued.
	RequestOTP(ctx context.Context, req RequestOTPRequest) (jobID string, err error)
	// VerifyOTP checks a submitted code. A wrong / expired / unknown code or too many attempts is
	// reported as verified=false with a nil error; only transport or server failures are errors.
	VerifyOTP(ctx context.Context, req VerifyOTPRequest) (verified bool, err error)
}

// RequestOTPRequest mirrors services/notification's otp domain.RequestOTP.
type RequestOTPRequest struct {
	Recipient string `json:"recipient"`
	Channel   string `json:"channel"` // email | push
	Purpose   string `json:"purpose"`
}

// VerifyOTPRequest mirrors services/notification's otp domain.RequestVerifyOTP.
type VerifyOTPRequest struct {
	Recipient string `json:"recipient"`
	Purpose   string `json:"purpose"`
	Code      string `json:"code"`
}

// SendNotificationRequest is the payload for SendNotification. Kept as a
// local, intentionally-duplicated copy of services/notification's request
// shape — sdk/ clients must not import a service's internal/ packages.
type SendNotificationRequest struct {
	Channel      string         `json:"channel"`
	TemplateCode string         `json:"templateCode"`
	Recipient    string         `json:"recipient"`
	Variables    map[string]any `json:"variables,omitempty"`
}

// GetNotificationLogsFilter is the query filter for GetAllNotificationLogs.
type GetNotificationLogsFilter struct {
	Page         int    `json:"page,omitempty"`
	Limit        int    `json:"limit,omitempty"`
	Channel      string `json:"channel,omitempty"`
	TemplateCode string `json:"templateCode,omitempty"`
	Recipient    string `json:"recipient,omitempty"`
	Status       string `json:"status,omitempty"`
}

// NotificationLogResponse mirrors services/notification's
// domain.ResponseNotificationLog JSON shape.
type NotificationLogResponse struct {
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

// NotificationLogListResponse mirrors services/notification's
// domain.ResponseNotificationLogList JSON shape.
type NotificationLogListResponse struct {
	Meta struct {
		Page         int `json:"page"`
		Limit        int `json:"limit"`
		TotalRecords int `json:"totalRecords"`
		TotalPages   int `json:"totalPages"`
	} `json:"meta"`
	Data []NotificationLogResponse `json:"data"`
}
