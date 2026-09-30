-- +goose Up
-- +goose StatementBegin
-- Asynchronous CSV exports: the table is the source of truth, the task queue only carries the job id.
CREATE TABLE IF NOT EXISTS export_jobs (
    "id" UUID NOT NULL PRIMARY KEY,
    "type" VARCHAR(20) NOT NULL,                 -- orders | order_lines | invoices
    "filter" JSONB NOT NULL DEFAULT '{}',
    "status" VARCHAR(20) NOT NULL,               -- queued | running | completed | failed | cancelled | expired
    "requested_by" VARCHAR(100) NOT NULL,
    "merchant_id" VARCHAR(100) NOT NULL DEFAULT '',
    "total_rows" BIGINT NOT NULL DEFAULT 0,
    "processed_rows" BIGINT NOT NULL DEFAULT 0,
    "file_name" VARCHAR(200) NOT NULL DEFAULT '',
    "file_size" BIGINT NOT NULL DEFAULT 0,
    "error" VARCHAR(500) NOT NULL DEFAULT '',
    "attempts" INT NOT NULL DEFAULT 0,
    "cancel_requested" BOOLEAN NOT NULL DEFAULT false,
    "heartbeat_at" TIMESTAMPTZ(6),
    "started_at" TIMESTAMPTZ(6),
    "finished_at" TIMESTAMPTZ(6),
    "expires_at" TIMESTAMPTZ(6),
    "created_at" TIMESTAMPTZ(6) NOT NULL DEFAULT now(),
    "updated_at" TIMESTAMPTZ(6) NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_export_jobs_requested ON export_jobs ("requested_by", "created_at");
CREATE INDEX IF NOT EXISTS idx_export_jobs_active ON export_jobs ("status", "updated_at") WHERE "status" IN ('queued', 'running');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS export_jobs;
-- +goose StatementEnd
