package domain

// Payment status of an order: a mirror of the payment service's status, fed by its payment.* events
// (or set by an admin override when an event was lost).
const (
	PaymentPending    = "pending"
	PaymentProcessing = "processing"
	PaymentPaid       = "paid"
	PaymentExpired    = "expired"
	PaymentCancelled  = "cancelled"
)

// IsFinalPayment reports whether a payment status can no longer change in the payment service
func IsFinalPayment(status string) bool {
	return status == PaymentPaid || status == PaymentExpired || status == PaymentCancelled
}

// Order status: the order's own lifecycle. Payment moves it out of awaiting_payment, every later
// step is made by staff.
const (
	OrderAwaitingPayment = "awaiting_payment"
	OrderConfirmed       = "confirmed"
	OrderPreparing       = "preparing"
	OrderReady           = "ready"
	OrderCompleted       = "completed"
	OrderCancelled       = "cancelled"
	OrderRefunded        = "refunded"
)

// Who made a change recorded in order_events / orders.payment_status_source
const (
	SourceKafka  = "kafka"
	SourceManual = "manual"
	SourceSystem = "system"
)

// Flags on a timeline entry
const (
	// FlagIgnoredManual a payment event that did not apply because an admin set a final payment status by hand
	FlagIgnoredManual = "ignored_manual"
	// FlagNeedsRefund money arrived for an order that was already cancelled
	FlagNeedsRefund = "needs_refund"
	// FlagStale an event older than the last applied one, recorded but not applied
	FlagStale = "stale"
)

// Payment events consumed from the payment service
const (
	EventPaymentCreated         = "payment.created"
	EventPaymentCheckoutStarted = "payment.checkout_started"
	EventPaymentCompleted       = "payment.completed"
	EventPaymentExpired         = "payment.expired"
	EventPaymentCancelled       = "payment.cancelled"
)

// Events this service publishes (topic = event name, `<service>.<event>`) and the manual timeline entries
const (
	EventOrderCreated            = "order.created"
	EventOrderConfirmed          = "order.confirmed"
	EventOrderStatusUpdated      = "order.status_updated"
	EventOrderCompleted          = "order.completed"
	EventOrderCancelled          = "order.cancelled"
	EventOrderRefunded           = "order.refunded"
	EventPaymentStatusOverridden = "order.payment_status_overridden"
	EventInvoiceIssued           = "order.invoice_issued"
	EventShiftOpened             = "order.shift_opened"
	EventShiftClosed             = "order.shift_closed"
	EventExportRequested         = "order.export_requested"
	EventExportCancelled         = "order.export_cancelled"
	EventExportDownloaded        = "order.export_downloaded"
	// EventNotificationEmail is an outbox event type whose topic comes from ORDER_NOTIFICATION_TOPIC
	EventNotificationEmail = "notification_email"
	// EventActivityLog is an outbox event type whose topic comes from ORDER_ACTIVITY_TOPIC
	EventActivityLog = "activity_log"
)

// MethodCash is the payment method code of cash payments (seeded by the payment service)
const MethodCash = "cash"

// DefaultMerchant is the merchant settings row used by merchants that have none of their own,
// and the merchant id of orders whose payment carries no merchantId
const DefaultMerchant = "*"

// Invoice types and statuses
const (
	InvoiceTypeInvoice    = "invoice"
	InvoiceTypeCreditNote = "credit_note"

	InvoiceIssued   = "issued"
	InvoiceCredited = "credited"
)

// Tax and rounding modes of merchant settings
const (
	TaxNone      = "none"
	TaxInclusive = "inclusive"
	TaxExclusive = "exclusive"

	RoundNone    = "none"
	RoundNearest = "nearest"
	RoundUp      = "up"
	RoundDown    = "down"
)

// Cash shift statuses
const (
	ShiftOpen   = "open"
	ShiftClosed = "closed"
)

// Export job types and statuses
const (
	ExportOrders     = "orders"
	ExportOrderLines = "order_lines"
	ExportInvoices   = "invoices"

	ExportQueued    = "queued"
	ExportRunning   = "running"
	ExportCompleted = "completed"
	ExportFailed    = "failed"
	ExportCancelled = "cancelled"
	ExportExpired   = "expired"
)
