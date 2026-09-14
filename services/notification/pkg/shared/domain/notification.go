package domain

import "time"

// NotificationTemplate model — an operator-editable message template
type NotificationTemplate struct {
	ID              int       `gorm:"column:id;primary_key" json:"id"`
	Code            string    `gorm:"column:code;type:varchar(100)" json:"code"`
	Channel         string    `gorm:"column:channel;type:varchar(20)" json:"channel"`
	SubjectTemplate *string   `gorm:"column:subject_template" json:"subjectTemplate,omitempty"`
	BodyTemplate    string    `gorm:"column:body_template" json:"bodyTemplate"`
	Variables       *string   `gorm:"column:variables" json:"variables,omitempty"`
	IsActive        bool      `gorm:"column:is_active" json:"isActive"`
	CreatedAt       time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt       time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName return table name of NotificationTemplate model
func (NotificationTemplate) TableName() string { return "notification_templates" }

// NotificationChannelConfig model — operator-editable per-channel settings
type NotificationChannelConfig struct {
	ID        int       `gorm:"column:id;primary_key" json:"id"`
	Channel   string    `gorm:"column:channel;type:varchar(20)" json:"channel"`
	IsEnabled bool      `gorm:"column:is_enabled" json:"isEnabled"`
	Settings  []byte    `gorm:"column:settings;type:jsonb" json:"settings,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName return table name of NotificationChannelConfig model
func (NotificationChannelConfig) TableName() string { return "notification_channel_configs" }

// NotificationLog model — a record of one dispatch attempt
type NotificationLog struct {
	ID              int        `gorm:"column:id;primary_key" json:"id"`
	Channel         string     `gorm:"column:channel;type:varchar(20)" json:"channel"`
	TemplateCode    string     `gorm:"column:template_code;type:varchar(100)" json:"templateCode"`
	Recipient       string     `gorm:"column:recipient;type:varchar(255)" json:"recipient"`
	RenderedSubject *string    `gorm:"column:rendered_subject" json:"renderedSubject,omitempty"`
	RenderedBody    *string    `gorm:"column:rendered_body" json:"renderedBody,omitempty"`
	Status          string     `gorm:"column:status;type:varchar(20)" json:"status"`
	ErrorMessage    *string    `gorm:"column:error_message" json:"errorMessage,omitempty"`
	SentAt          *time.Time `gorm:"column:sent_at" json:"sentAt,omitempty"`
	CreatedAt       time.Time  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt       time.Time  `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName return table name of NotificationLog model
func (NotificationLog) TableName() string { return "notification_logs" }
