package domain

// Payment.Status values
const (
	PaymentPending    = "pending"
	PaymentProcessing = "processing"
	PaymentPaid       = "paid"
	PaymentExpired    = "expired"
	PaymentCancelled  = "cancelled"
)

// Transaction.Status values
const (
	TransactionPending   = "pending"
	TransactionPaid      = "paid"
	TransactionFailed    = "failed"
	TransactionExpired   = "expired"
	TransactionCancelled = "cancelled"
)

// Method.Type values
const (
	MethodVirtualAccount = "virtual_account"
	MethodQRIS           = "qris"
	MethodEWallet        = "ewallet"
	MethodCard           = "card"
	MethodRetail         = "retail"
	MethodCash           = "cash"
)

// Gateway codes
const (
	GatewayMidtrans = "midtrans"
	GatewayXendit   = "xendit"
	GatewayMock     = "mock"
)

// Topic directions
const (
	TopicConsume = "consume"
	TopicPublish = "publish"
)

// Outbound event types, resolved to a topic through payment_kafka_topics.
const (
	EventPaymentCreated = "payment.created"
	// EventCheckoutStarted is published for every checkout attempt (each pay action), before the gateway is called
	EventCheckoutStarted  = "payment.checkout_started"
	EventPaymentCompleted = "payment.completed"
	EventPaymentExpired   = "payment.expired"
	EventPaymentCancelled = "payment.cancelled"
	EventNotificationMail = "notification_email"
)

// CallbackLog.Status values
const (
	CallbackProcessed = "processed"
	CallbackDuplicate = "duplicate"
	CallbackIgnored   = "ignored"
	CallbackFailed    = "failed"
)

// Gateway environments
const (
	EnvSandbox    = "sandbox"
	EnvProduction = "production"
)
