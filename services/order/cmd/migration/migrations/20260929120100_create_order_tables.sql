-- +goose Up
-- +goose StatementBegin
-- One order per payment of the payment service, built from its payment.* events.
CREATE TABLE IF NOT EXISTS orders (
    "id" BIGSERIAL NOT NULL PRIMARY KEY,
    "order_number" VARCHAR(60) NOT NULL,
    "payment_id" UUID NOT NULL,
    "source" VARCHAR(100) NOT NULL DEFAULT '',
    "reference_id" VARCHAR(100) NOT NULL DEFAULT '',
    "merchant_id" VARCHAR(100) NOT NULL DEFAULT '*',
    "outlet_id" VARCHAR(100) NOT NULL DEFAULT '',
    "cashier_id" VARCHAR(100) NOT NULL DEFAULT '',
    "channel" VARCHAR(50) NOT NULL DEFAULT '',
    "description" VARCHAR(500) NOT NULL DEFAULT '',
    "customer_name" VARCHAR(200) NOT NULL DEFAULT '',
    "customer_email" VARCHAR(200) NOT NULL DEFAULT '',
    "customer_phone" VARCHAR(30) NOT NULL DEFAULT '',
    "currency" VARCHAR(3) NOT NULL DEFAULT 'IDR',
    "subtotal" BIGINT NOT NULL DEFAULT 0,
    "tax_amount" BIGINT NOT NULL DEFAULT 0,
    "rounding_adjustment" BIGINT NOT NULL DEFAULT 0,
    "expected_total" BIGINT NOT NULL DEFAULT 0,
    "amount" BIGINT NOT NULL DEFAULT 0,
    "fee" BIGINT NOT NULL DEFAULT 0,
    "total_amount" BIGINT NOT NULL DEFAULT 0,
    "amount_mismatch" BOOLEAN NOT NULL DEFAULT false,
    "tax_name" VARCHAR(30) NOT NULL DEFAULT '',
    "tax_rate" NUMERIC(5,2) NOT NULL DEFAULT 0,
    "tax_mode" VARCHAR(20) NOT NULL DEFAULT 'none',
    "rounding_mode" VARCHAR(20) NOT NULL DEFAULT 'none',
    "rounding_unit" BIGINT NOT NULL DEFAULT 1,
    "payment_status" VARCHAR(20) NOT NULL,
    "payment_status_source" VARCHAR(10) NOT NULL DEFAULT 'kafka',
    "order_status" VARCHAR(20) NOT NULL,
    "method_code" VARCHAR(50) NOT NULL DEFAULT '',
    "gateway_code" VARCHAR(50) NOT NULL DEFAULT '',
    "transaction_id" VARCHAR(64) NOT NULL DEFAULT '',
    "attempt_count" INT NOT NULL DEFAULT 0,
    "shift_id" BIGINT,
    "refund_shift_id" BIGINT,
    "metadata" JSONB NOT NULL DEFAULT '{}',
    "placed_at" TIMESTAMPTZ(6) NOT NULL,
    "paid_at" TIMESTAMPTZ(6),
    "expires_at" TIMESTAMPTZ(6),
    "confirmed_at" TIMESTAMPTZ(6),
    "completed_at" TIMESTAMPTZ(6),
    "cancelled_at" TIMESTAMPTZ(6),
    "refunded_at" TIMESTAMPTZ(6),
    "last_event_at" TIMESTAMPTZ(6),
    "version" INT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ(6) NOT NULL DEFAULT now(),
    "updated_at" TIMESTAMPTZ(6) NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_orders_payment_id ON orders ("payment_id");
CREATE UNIQUE INDEX IF NOT EXISTS idx_orders_order_number ON orders ("order_number");
CREATE INDEX IF NOT EXISTS idx_orders_source_ref ON orders ("source", "reference_id");
CREATE INDEX IF NOT EXISTS idx_orders_merchant_outlet_placed ON orders ("merchant_id", "outlet_id", "placed_at");
CREATE INDEX IF NOT EXISTS idx_orders_payment_status ON orders ("payment_status", "placed_at");
CREATE INDEX IF NOT EXISTS idx_orders_order_status ON orders ("order_status", "placed_at");
CREATE INDEX IF NOT EXISTS idx_orders_shift ON orders ("shift_id") WHERE "shift_id" IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_orders_refund_shift ON orders ("refund_shift_id") WHERE "refund_shift_id" IS NOT NULL;

CREATE TABLE IF NOT EXISTS order_items (
    "id" BIGSERIAL NOT NULL PRIMARY KEY,
    "order_id" BIGINT NOT NULL REFERENCES orders ("id") ON DELETE CASCADE,
    "line_no" INT NOT NULL,
    "name" VARCHAR(200) NOT NULL,
    "price" BIGINT NOT NULL,
    "quantity" INT NOT NULL,
    "line_total" BIGINT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_order_items_line ON order_items ("order_id", "line_no");

-- Timeline and audit trail: payment events consumed (source kafka) and manual changes.
CREATE TABLE IF NOT EXISTS order_events (
    "id" BIGSERIAL NOT NULL PRIMARY KEY,
    "order_id" BIGINT NOT NULL REFERENCES orders ("id") ON DELETE CASCADE,
    "event" VARCHAR(60) NOT NULL,
    "source" VARCHAR(10) NOT NULL,                -- kafka | manual | system
    "transaction_id" VARCHAR(64) NOT NULL DEFAULT '',
    "from_status" VARCHAR(20) NOT NULL DEFAULT '',
    "to_status" VARCHAR(20) NOT NULL DEFAULT '',
    "amount" BIGINT NOT NULL DEFAULT 0,
    "fee" BIGINT NOT NULL DEFAULT 0,
    "total_amount" BIGINT NOT NULL DEFAULT 0,
    "method_code" VARCHAR(50) NOT NULL DEFAULT '',
    "actor" VARCHAR(100) NOT NULL DEFAULT '',
    "note" VARCHAR(500) NOT NULL DEFAULT '',
    "flag" VARCHAR(30) NOT NULL DEFAULT '',       -- ignored_manual | needs_refund | stale
    "payload" JSONB NOT NULL DEFAULT '{}',
    "topic" VARCHAR(200) NOT NULL DEFAULT '',
    "partition" INT NOT NULL DEFAULT 0,
    "offset" BIGINT NOT NULL DEFAULT 0,
    "occurred_at" TIMESTAMPTZ(6) NOT NULL,
    "created_at" TIMESTAMPTZ(6) NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_order_events_order ON order_events ("order_id", "occurred_at");
-- a payment event redelivered by Kafka is recorded once
CREATE UNIQUE INDEX IF NOT EXISTS idx_order_events_dedupe ON order_events ("order_id", "event", "transaction_id") WHERE "source" = 'kafka';

CREATE TABLE IF NOT EXISTS order_outbox (
    "id" BIGSERIAL NOT NULL PRIMARY KEY,
    "event_type" VARCHAR(60) NOT NULL,
    "topic" VARCHAR(200) NOT NULL,
    "key" VARCHAR(100) NOT NULL DEFAULT '',
    "payload" JSONB NOT NULL,
    "attempts" INT NOT NULL DEFAULT 0,
    "last_error" VARCHAR(500) NOT NULL DEFAULT '',
    "published_at" TIMESTAMPTZ(6),
    "created_at" TIMESTAMPTZ(6) NOT NULL DEFAULT now(),
    "updated_at" TIMESTAMPTZ(6) NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_order_outbox_pending ON order_outbox ("id") WHERE "published_at" IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS order_outbox;
DROP TABLE IF EXISTS order_events;
DROP TABLE IF EXISTS order_items;
DROP TABLE IF EXISTS orders;
-- +goose StatementEnd
