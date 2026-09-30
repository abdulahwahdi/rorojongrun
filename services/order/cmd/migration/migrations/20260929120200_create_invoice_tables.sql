-- +goose Up
-- +goose StatementBegin
-- Sales invoices (issued when an order's payment is paid) and the credit notes that reverse them.
CREATE TABLE IF NOT EXISTS invoices (
    "id" BIGSERIAL NOT NULL PRIMARY KEY,
    "number" VARCHAR(80) NOT NULL,
    "type" VARCHAR(20) NOT NULL,                  -- invoice | credit_note
    "ref_invoice_id" BIGINT REFERENCES invoices ("id"),
    "order_id" BIGINT NOT NULL REFERENCES orders ("id"),
    "order_number" VARCHAR(60) NOT NULL,
    "status" VARCHAR(20) NOT NULL,                -- issued | credited
    "merchant_id" VARCHAR(100) NOT NULL,
    "outlet_id" VARCHAR(100) NOT NULL DEFAULT '',
    "seller_name" VARCHAR(200) NOT NULL DEFAULT '',
    "seller_address" VARCHAR(500) NOT NULL DEFAULT '',
    "seller_tax_id" VARCHAR(50) NOT NULL DEFAULT '',
    "customer_name" VARCHAR(200) NOT NULL DEFAULT '',
    "customer_email" VARCHAR(200) NOT NULL DEFAULT '',
    "customer_phone" VARCHAR(30) NOT NULL DEFAULT '',
    "currency" VARCHAR(3) NOT NULL DEFAULT 'IDR',
    "subtotal" BIGINT NOT NULL DEFAULT 0,
    "tax_name" VARCHAR(30) NOT NULL DEFAULT '',
    "tax_rate" NUMERIC(5,2) NOT NULL DEFAULT 0,
    "tax_mode" VARCHAR(20) NOT NULL DEFAULT 'none',
    "tax_amount" BIGINT NOT NULL DEFAULT 0,
    "rounding_adjustment" BIGINT NOT NULL DEFAULT 0,
    "fee" BIGINT NOT NULL DEFAULT 0,
    "total_amount" BIGINT NOT NULL DEFAULT 0,
    "method_code" VARCHAR(50) NOT NULL DEFAULT '',
    "paid_at" TIMESTAMPTZ(6),
    "issued_at" TIMESTAMPTZ(6) NOT NULL,
    "emailed_at" TIMESTAMPTZ(6),
    "created_at" TIMESTAMPTZ(6) NOT NULL DEFAULT now(),
    "updated_at" TIMESTAMPTZ(6) NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_invoices_number ON invoices ("number");
-- exactly one invoice and at most one credit note per order
CREATE UNIQUE INDEX IF NOT EXISTS idx_invoices_order_type ON invoices ("order_id", "type");
CREATE INDEX IF NOT EXISTS idx_invoices_merchant_issued ON invoices ("merchant_id", "outlet_id", "issued_at");

CREATE TABLE IF NOT EXISTS invoice_items (
    "id" BIGSERIAL NOT NULL PRIMARY KEY,
    "invoice_id" BIGINT NOT NULL REFERENCES invoices ("id") ON DELETE CASCADE,
    "line_no" INT NOT NULL,
    "name" VARCHAR(200) NOT NULL,
    "price" BIGINT NOT NULL,
    "quantity" INT NOT NULL,
    "line_total" BIGINT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_invoice_items_line ON invoice_items ("invoice_id", "line_no");
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS invoice_items;
DROP TABLE IF EXISTS invoices;
-- +goose StatementEnd
