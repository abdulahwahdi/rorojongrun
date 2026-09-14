-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS notification_templates (
    "id" SERIAL NOT NULL PRIMARY KEY,
    "code" VARCHAR(100) NOT NULL,
    "channel" VARCHAR(20) NOT NULL, -- 'email' | 'push'
    "subject_template" TEXT,        -- nullable, push has no subject
    "body_template" TEXT NOT NULL,
    "variables" TEXT,               -- documentation aid: expected placeholder names
    "is_active" BOOLEAN NOT NULL DEFAULT true,
    "created_at" TIMESTAMPTZ(6),
    "updated_at" TIMESTAMPTZ(6)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_notification_templates_code ON notification_templates ("code");

CREATE TABLE IF NOT EXISTS notification_channel_configs (
    "id" SERIAL NOT NULL PRIMARY KEY,
    "channel" VARCHAR(20) NOT NULL,
    "is_enabled" BOOLEAN NOT NULL DEFAULT true,
    "settings" JSONB,                -- non-secret settings only (from_address, project_id, ...)
    "created_at" TIMESTAMPTZ(6),
    "updated_at" TIMESTAMPTZ(6)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_notification_channel_configs_channel ON notification_channel_configs ("channel");

CREATE TABLE IF NOT EXISTS notification_logs (
    "id" SERIAL NOT NULL PRIMARY KEY,
    "channel" VARCHAR(20) NOT NULL,
    "template_code" VARCHAR(100) NOT NULL,
    "recipient" VARCHAR(255) NOT NULL,
    "rendered_subject" TEXT,
    "rendered_body" TEXT,
    "status" VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending|sent|failed
    "error_message" TEXT,
    "sent_at" TIMESTAMPTZ(6),
    "created_at" TIMESTAMPTZ(6),
    "updated_at" TIMESTAMPTZ(6)
);
CREATE INDEX IF NOT EXISTS idx_notification_logs_recipient ON notification_logs ("recipient");
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS notification_logs;
DROP TABLE IF EXISTS notification_channel_configs;
DROP TABLE IF EXISTS notification_templates;
-- +goose StatementEnd
