-- +goose Up
-- +goose StatementBegin
-- Emails the payment service asks for through notification.requested (see services/payment/docs/payment-flow.md).
-- Operators can edit these afterwards through the template API; re-running this migration never overwrites edits.
INSERT INTO notification_templates ("code", "channel", "subject_template", "body_template", "variables", "is_active", "created_at", "updated_at") VALUES
(
    'payment_checkout', 'email',
    'Complete your payment for {{.referenceId}}',
    '<p>Hi {{.customerName}},</p>
<p>Your payment for <b>{{.referenceId}}</b> is waiting.</p>
<p>Amount to pay: <b>{{.totalAmount}}</b> via {{.methodName}}<br>Pay before: {{.expiresAt}}</p>
<ul>{{range .instructionLines}}<li>{{.}}</li>{{end}}</ul>
<p><a href="{{.paymentUrl}}">Open your payment page</a></p>',
    'customerName, referenceId, description, totalAmount, amount, fee, methodName, expiresAt, paymentUrl, instructionLines (list)',
    true, now(), now()
),
(
    'payment_paid', 'email',
    'Payment received for {{.referenceId}}',
    '<p>Hi {{.customerName}},</p>
<p>We received your payment of <b>{{.totalAmount}}</b> for <b>{{.referenceId}}</b> via {{.methodName}} on {{.paidAt}}.</p>
<p>Thank you!</p>',
    'customerName, referenceId, description, totalAmount, amount, fee, methodName, paidAt',
    true, now(), now()
)
ON CONFLICT ("code") DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM notification_templates WHERE "code" IN ('payment_checkout', 'payment_paid');
-- +goose StatementEnd
