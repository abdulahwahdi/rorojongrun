-- +goose Up
-- +goose StatementBegin
-- A cashier's shift at an outlet: opening float, cash sales and refunds, and the count at closing.
CREATE TABLE IF NOT EXISTS cash_shifts (
    "id" BIGSERIAL NOT NULL PRIMARY KEY,
    "merchant_id" VARCHAR(100) NOT NULL,
    "outlet_id" VARCHAR(100) NOT NULL DEFAULT '',
    "cashier_id" VARCHAR(100) NOT NULL,
    "status" VARCHAR(10) NOT NULL,               -- open | closed
    "opening_float" BIGINT NOT NULL DEFAULT 0,
    "opened_at" TIMESTAMPTZ(6) NOT NULL,
    "opened_by" VARCHAR(100) NOT NULL DEFAULT '',
    "closed_at" TIMESTAMPTZ(6),
    "closed_by" VARCHAR(100) NOT NULL DEFAULT '',
    "cash_sales" BIGINT NOT NULL DEFAULT 0,
    "cash_refunds" BIGINT NOT NULL DEFAULT 0,
    "order_count" INT NOT NULL DEFAULT 0,
    "expected_cash" BIGINT NOT NULL DEFAULT 0,
    "counted_cash" BIGINT,
    "difference" BIGINT,
    "note" VARCHAR(500) NOT NULL DEFAULT '',
    "created_at" TIMESTAMPTZ(6) NOT NULL DEFAULT now(),
    "updated_at" TIMESTAMPTZ(6) NOT NULL DEFAULT now()
);
-- one open shift per cashier and outlet
CREATE UNIQUE INDEX IF NOT EXISTS idx_cash_shifts_open ON cash_shifts ("merchant_id", "outlet_id", "cashier_id") WHERE "status" = 'open';
CREATE INDEX IF NOT EXISTS idx_cash_shifts_opened ON cash_shifts ("merchant_id", "outlet_id", "opened_at");
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS cash_shifts;
-- +goose StatementEnd
