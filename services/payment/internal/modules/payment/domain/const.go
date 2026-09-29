package domain

// Cron job names
const (
	JobExpirePayments = "payment-expire"
	JobFlushOutbox    = "payment-flush-outbox"
)

// Email template codes seeded in the notification service
const (
	TemplateCheckout = "payment_checkout"
	TemplatePaid     = "payment_paid"
)

// Limits for a payment link's lifetime, in seconds
const (
	MinExpirySec = 60
	MaxExpirySec = 30 * 24 * 3600
)
