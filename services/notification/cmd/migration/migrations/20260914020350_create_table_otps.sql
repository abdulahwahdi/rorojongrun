-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS otp_requests (
    "id" SERIAL NOT NULL PRIMARY KEY,
    "recipient" VARCHAR(255) NOT NULL,
    "channel" VARCHAR(20) NOT NULL,       -- 'email' | 'push'
    "purpose" VARCHAR(50) NOT NULL,       -- e.g. 'login', 'reset_password'
    "code_hash" VARCHAR(255) NOT NULL,
    "expires_at" TIMESTAMPTZ(6) NOT NULL,
    "max_attempts" INTEGER NOT NULL DEFAULT 5,
    "attempt_count" INTEGER NOT NULL DEFAULT 0,
    "verified_at" TIMESTAMPTZ(6),
    "created_at" TIMESTAMPTZ(6),
    "updated_at" TIMESTAMPTZ(6)
);
CREATE INDEX IF NOT EXISTS idx_otp_requests_recipient_purpose ON otp_requests ("recipient", "purpose");
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS otp_requests;
-- +goose StatementEnd
