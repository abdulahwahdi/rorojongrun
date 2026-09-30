-- +goose Up
-- +goose StatementBegin
-- Emails the order service asks for through notification.requested when it issues an invoice or a
-- credit note (see services/order/docs/order-ledger.md). Operators can edit these afterwards through
-- the template API; re-running this migration never overwrites edits.
INSERT INTO notification_templates ("code", "channel", "subject_template", "body_template", "variables", "is_active", "created_at", "updated_at") VALUES
(
    'order_invoice', 'email',
    'Your invoice {{.invoiceNumber}} from {{.sellerName}}',
    '<p>Hi {{.customerName}},</p>
<p>Thank you for your order <b>{{.orderNumber}}</b>. Here is your invoice <b>{{.invoiceNumber}}</b> ({{.issuedAt}}).</p>
<ul>{{range .lines}}<li>{{.}}</li>{{end}}</ul>
<p>Subtotal: {{.subtotal}}<br>{{.taxName}}: {{.taxAmount}}<br>Rounding: {{.roundingAdjustment}}<br>Fee: {{.fee}}<br><b>Total paid: {{.totalAmount}}</b> ({{.methodCode}})</p>
<p>{{.sellerName}}<br>{{.sellerAddress}}<br>{{.sellerTaxId}}</p>',
    'customerName, invoiceNumber, orderNumber, sellerName, sellerAddress, sellerTaxId, issuedAt, subtotal, taxName, taxAmount, roundingAdjustment, fee, totalAmount, methodCode, lines (list)',
    true, now(), now()
),
(
    'order_credit_note', 'email',
    'Credit note {{.invoiceNumber}} for your order {{.orderNumber}}',
    '<p>Hi {{.customerName}},</p>
<p>Your order <b>{{.orderNumber}}</b> has been refunded. Credit note <b>{{.invoiceNumber}}</b> ({{.issuedAt}}) reverses invoice {{.refInvoiceNumber}}.</p>
<ul>{{range .lines}}<li>{{.}}</li>{{end}}</ul>
<p><b>Refunded: {{.totalAmount}}</b> ({{.methodCode}})</p>
<p>{{.sellerName}}<br>{{.sellerAddress}}</p>',
    'customerName, invoiceNumber, refInvoiceNumber, orderNumber, sellerName, sellerAddress, sellerTaxId, issuedAt, subtotal, taxName, taxAmount, roundingAdjustment, fee, totalAmount, methodCode, lines (list)',
    true, now(), now()
)
ON CONFLICT ("code") DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM notification_templates WHERE "code" IN ('order_invoice', 'order_credit_note');
-- +goose StatementEnd
