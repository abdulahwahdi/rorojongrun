-- +goose Up
-- +goose StatementBegin
-- Per-merchant numbering, tax, rounding and invoice seller data. The '*' row is the default for
-- merchants without their own row (and for orders whose payment carries no merchantId).
CREATE TABLE IF NOT EXISTS merchant_settings (
    "merchant_id" VARCHAR(100) NOT NULL PRIMARY KEY,
    "name" VARCHAR(200) NOT NULL DEFAULT '',
    "address" VARCHAR(500) NOT NULL DEFAULT '',
    "tax_id" VARCHAR(50) NOT NULL DEFAULT '',             -- e.g. NPWP, printed on invoices
    "order_prefix" VARCHAR(10) NOT NULL DEFAULT 'ORD',
    "invoice_prefix" VARCHAR(10) NOT NULL DEFAULT 'INV',
    "credit_note_prefix" VARCHAR(10) NOT NULL DEFAULT 'CN',
    "tax_name" VARCHAR(30) NOT NULL DEFAULT 'PPN',
    "tax_rate" NUMERIC(5,2) NOT NULL DEFAULT 0,           -- percent, e.g. 11.00
    "tax_mode" VARCHAR(20) NOT NULL DEFAULT 'none',       -- none | inclusive | exclusive
    "rounding_mode" VARCHAR(20) NOT NULL DEFAULT 'none',  -- none | nearest | up | down
    "rounding_unit" BIGINT NOT NULL DEFAULT 1,            -- e.g. 100 rounds totals to Rp100
    "timezone" VARCHAR(64) NOT NULL DEFAULT 'Asia/Jakarta', -- number periods and report days
    "created_at" TIMESTAMPTZ(6) NOT NULL DEFAULT now(),
    "updated_at" TIMESTAMPTZ(6) NOT NULL DEFAULT now(),
    CONSTRAINT chk_merchant_settings_tax_mode CHECK ("tax_mode" IN ('none', 'inclusive', 'exclusive')),
    CONSTRAINT chk_merchant_settings_rounding CHECK ("rounding_mode" IN ('none', 'nearest', 'up', 'down') AND "rounding_unit" >= 1)
);

INSERT INTO merchant_settings ("merchant_id", "name") VALUES ('*', 'RoRoJongRun')
ON CONFLICT ("merchant_id") DO NOTHING;

-- The last number handed out per numbering scope (e.g. 'order:M1', 'invoice:M1') and period
-- (a day for orders, a month for invoices), locked FOR UPDATE so numbers have no gaps.
CREATE TABLE IF NOT EXISTS number_sequences (
    "scope" VARCHAR(150) NOT NULL,
    "period" VARCHAR(20) NOT NULL,
    "last_no" BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY ("scope", "period")
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS number_sequences;
DROP TABLE IF EXISTS merchant_settings;
-- +goose StatementEnd
