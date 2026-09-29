-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS payment_gateways (
    "id" SERIAL NOT NULL PRIMARY KEY,
    "code" VARCHAR(50) NOT NULL,                 -- midtrans | xendit | mock
    "name" VARCHAR(100) NOT NULL,
    "is_enabled" BOOLEAN NOT NULL DEFAULT false,
    "environment" VARCHAR(20) NOT NULL DEFAULT 'sandbox', -- sandbox | production
    "settings" JSONB NOT NULL DEFAULT '{}',      -- non-secret: baseUrl, merchantId, ...
    "credentials_enc" TEXT NOT NULL DEFAULT '',  -- AES-GCM(json of secrets), write-only via API
    "created_at" TIMESTAMPTZ(6),
    "updated_at" TIMESTAMPTZ(6)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_payment_gateways_code ON payment_gateways ("code");

CREATE TABLE IF NOT EXISTS payment_methods (
    "id" SERIAL NOT NULL PRIMARY KEY,
    "code" VARCHAR(50) NOT NULL,
    "name" VARCHAR(100) NOT NULL,
    "type" VARCHAR(30) NOT NULL,                 -- virtual_account | qris | ewallet | card | retail | cash
    "gateway_code" VARCHAR(50),                  -- NULL for cash
    "gateway_channel" VARCHAR(50) NOT NULL DEFAULT '', -- provider specific channel, e.g. bca, gopay
    "is_enabled" BOOLEAN NOT NULL DEFAULT true,
    "icon_url" VARCHAR(500) NOT NULL DEFAULT '',
    "sort_order" INT NOT NULL DEFAULT 0,
    "min_amount" BIGINT NOT NULL DEFAULT 0,
    "max_amount" BIGINT NOT NULL DEFAULT 0,      -- 0 = no upper limit
    "fee_flat" BIGINT NOT NULL DEFAULT 0,
    "fee_percent" NUMERIC(6,3) NOT NULL DEFAULT 0,
    "instructions" TEXT NOT NULL DEFAULT '',
    "created_at" TIMESTAMPTZ(6),
    "updated_at" TIMESTAMPTZ(6)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_payment_methods_code ON payment_methods ("code");

CREATE TABLE IF NOT EXISTS payment_kafka_topics (
    "id" SERIAL NOT NULL PRIMARY KEY,
    "topic" VARCHAR(200) NOT NULL,
    "direction" VARCHAR(10) NOT NULL,            -- consume | publish
    "gateway_code" VARCHAR(50),                  -- consume: which gateway's callbacks arrive here
    "event_type" VARCHAR(50),                    -- publish: payment.created | payment.completed | ... | notification_email
    "is_enabled" BOOLEAN NOT NULL DEFAULT true,
    "created_at" TIMESTAMPTZ(6),
    "updated_at" TIMESTAMPTZ(6)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_payment_kafka_topics_topic_dir ON payment_kafka_topics ("topic", "direction");

CREATE TABLE IF NOT EXISTS payments (
    "id" UUID NOT NULL PRIMARY KEY,
    "token" VARCHAR(64) NOT NULL,                -- unguessable checkout credential
    "source" VARCHAR(100) NOT NULL,              -- calling service, e.g. order
    "reference_id" VARCHAR(100) NOT NULL,        -- caller's own id, e.g. order id
    "description" VARCHAR(500) NOT NULL DEFAULT '',
    "amount" BIGINT NOT NULL,                    -- smallest currency unit as-is (IDR: rupiah)
    "fee" BIGINT NOT NULL DEFAULT 0,
    "total_amount" BIGINT NOT NULL,
    "currency" VARCHAR(10) NOT NULL DEFAULT 'IDR',
    "status" VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending|processing|paid|expired|cancelled
    "method_code" VARCHAR(50) NOT NULL DEFAULT '',
    "customer" JSONB NOT NULL DEFAULT '{}',
    "items" JSONB NOT NULL DEFAULT '[]',
    "metadata" JSONB NOT NULL DEFAULT '{}',
    "allowed_methods" JSONB NOT NULL DEFAULT '[]',
    "success_url" VARCHAR(1000) NOT NULL DEFAULT '',
    "failure_url" VARCHAR(1000) NOT NULL DEFAULT '',
    "expires_at" TIMESTAMPTZ(6) NOT NULL,
    "paid_at" TIMESTAMPTZ(6),
    "created_at" TIMESTAMPTZ(6),
    "updated_at" TIMESTAMPTZ(6)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_payments_token ON payments ("token");
CREATE UNIQUE INDEX IF NOT EXISTS idx_payments_active_ref ON payments ("source", "reference_id") WHERE "status" IN ('pending', 'processing');
CREATE INDEX IF NOT EXISTS idx_payments_source_ref ON payments ("source", "reference_id");
CREATE INDEX IF NOT EXISTS idx_payments_expiry ON payments ("expires_at") WHERE "status" IN ('pending', 'processing');

CREATE TABLE IF NOT EXISTS payment_transactions (
    "id" UUID NOT NULL PRIMARY KEY,              -- also the order_id / external_id sent to the gateway
    "payment_id" UUID NOT NULL REFERENCES payments("id"),
    "method_code" VARCHAR(50) NOT NULL,
    "gateway_code" VARCHAR(50) NOT NULL DEFAULT '', -- '' for cash
    "status" VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending|paid|failed|expired|cancelled
    "amount" BIGINT NOT NULL,                    -- payment amount + fee, what the customer pays
    "instruction" JSONB NOT NULL DEFAULT '{}',   -- VA number / QR string / redirect url / cash code
    "gateway_response" JSONB NOT NULL DEFAULT '{}',
    "cash_code" VARCHAR(20),
    "cash_received" BIGINT,
    "confirmed_by" VARCHAR(100),
    "expires_at" TIMESTAMPTZ(6) NOT NULL,
    "paid_at" TIMESTAMPTZ(6),
    "failure_reason" VARCHAR(500) NOT NULL DEFAULT '',
    "created_at" TIMESTAMPTZ(6),
    "updated_at" TIMESTAMPTZ(6)
);
CREATE INDEX IF NOT EXISTS idx_payment_transactions_payment ON payment_transactions ("payment_id");
CREATE UNIQUE INDEX IF NOT EXISTS idx_payment_transactions_cash_code ON payment_transactions ("cash_code") WHERE "cash_code" IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_payment_transactions_expiry ON payment_transactions ("expires_at") WHERE "status" = 'pending';

CREATE TABLE IF NOT EXISTS payment_callback_logs (
    "id" SERIAL NOT NULL PRIMARY KEY,
    "topic" VARCHAR(200) NOT NULL,
    "partition" INT NOT NULL DEFAULT 0,
    "offset" BIGINT NOT NULL DEFAULT 0,
    "gateway_code" VARCHAR(50) NOT NULL DEFAULT '',
    "external_id" VARCHAR(100) NOT NULL DEFAULT '',
    "headers" JSONB NOT NULL DEFAULT '{}',
    "payload" TEXT NOT NULL DEFAULT '',
    "status" VARCHAR(20) NOT NULL,               -- processed|duplicate|ignored|failed
    "error" TEXT NOT NULL DEFAULT '',
    "transaction_id" UUID,
    "created_at" TIMESTAMPTZ(6),
    "updated_at" TIMESTAMPTZ(6)
);
CREATE INDEX IF NOT EXISTS idx_payment_callback_logs_topic_offset ON payment_callback_logs ("topic", "partition", "offset");
CREATE INDEX IF NOT EXISTS idx_payment_callback_logs_external ON payment_callback_logs ("external_id");

CREATE TABLE IF NOT EXISTS payment_outbox (
    "id" BIGSERIAL NOT NULL PRIMARY KEY,
    "event_type" VARCHAR(50) NOT NULL,           -- resolved to a topic via payment_kafka_topics at publish time
    "key" VARCHAR(100) NOT NULL DEFAULT '',
    "payload" JSONB NOT NULL,
    "attempts" INT NOT NULL DEFAULT 0,
    "last_error" TEXT NOT NULL DEFAULT '',
    "published_at" TIMESTAMPTZ(6),
    "created_at" TIMESTAMPTZ(6),
    "updated_at" TIMESTAMPTZ(6)
);
CREATE INDEX IF NOT EXISTS idx_payment_outbox_pending ON payment_outbox ("id") WHERE "published_at" IS NULL;

-- seed: gateways (disabled until an operator adds credentials)
INSERT INTO payment_gateways ("code", "name", "is_enabled", "environment", "created_at", "updated_at") VALUES
    ('midtrans', 'Midtrans', false, 'sandbox', now(), now()),
    ('xendit',   'Xendit',   false, 'sandbox', now(), now()),
    ('mock',     'Mock Gateway (dev only)', false, 'sandbox', now(), now())
ON CONFLICT ("code") DO NOTHING;

-- seed: methods
INSERT INTO payment_methods ("code", "name", "type", "gateway_code", "gateway_channel", "is_enabled", "sort_order", "instructions", "created_at", "updated_at") VALUES
    ('cash',          'Cash',                'cash',            NULL,       '',         true,  1,  'Show the payment code to the cashier and pay in cash.', now(), now()),
    ('bca_va',        'BCA Virtual Account', 'virtual_account', 'midtrans', 'bca',      true,  10, '', now(), now()),
    ('bni_va',        'BNI Virtual Account', 'virtual_account', 'midtrans', 'bni',      true,  11, '', now(), now()),
    ('bri_va',        'BRI Virtual Account', 'virtual_account', 'midtrans', 'bri',      true,  12, '', now(), now()),
    ('permata_va',    'Permata Virtual Account', 'virtual_account', 'midtrans', 'permata', true, 13, '', now(), now()),
    ('mandiri_bill',  'Mandiri Bill Payment', 'virtual_account', 'midtrans', 'mandiri', true,  14, '', now(), now()),
    ('qris',          'QRIS',                'qris',            'midtrans', 'qris',     true,  20, '', now(), now()),
    ('gopay',         'GoPay',               'ewallet',         'midtrans', 'gopay',    true,  30, '', now(), now()),
    ('shopeepay',     'ShopeePay',           'ewallet',         'midtrans', 'shopeepay', true, 31, '', now(), now()),
    ('credit_card',   'Credit / Debit Card', 'card',            'midtrans', 'credit_card', true, 40, '', now(), now()),
    ('xendit_invoice','Xendit Invoice (all channels)', 'virtual_account', 'xendit', 'invoice', true, 50, '', now(), now()),
    ('mock_va',       'Mock Virtual Account (dev only)', 'virtual_account', 'mock', 'va', true, 90, '', now(), now())
ON CONFLICT ("code") DO NOTHING;

-- seed: kafka topics
INSERT INTO payment_kafka_topics ("topic", "direction", "gateway_code", "event_type", "is_enabled", "created_at", "updated_at") VALUES
    ('payment.midtrans_callback_received', 'consume', 'midtrans', NULL, true, now(), now()),
    ('payment.xendit_callback_received',   'consume', 'xendit',   NULL, true, now(), now()),
    ('payment.mock_callback_received',     'consume', 'mock',     NULL, true, now(), now()),
    ('payment.created',   'publish', NULL, 'payment.created',   true, now(), now()),
    ('payment.checkout_started', 'publish', NULL, 'payment.checkout_started', true, now(), now()),
    ('payment.completed', 'publish', NULL, 'payment.completed', true, now(), now()),
    ('payment.expired',   'publish', NULL, 'payment.expired',   true, now(), now()),
    ('payment.cancelled', 'publish', NULL, 'payment.cancelled', true, now(), now()),
    ('notification.requested', 'publish', NULL, 'notification_email', true, now(), now())
ON CONFLICT ("topic", "direction") DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS payment_outbox;
DROP TABLE IF EXISTS payment_callback_logs;
DROP TABLE IF EXISTS payment_transactions;
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS payment_kafka_topics;
DROP TABLE IF EXISTS payment_methods;
DROP TABLE IF EXISTS payment_gateways;
-- +goose StatementEnd
