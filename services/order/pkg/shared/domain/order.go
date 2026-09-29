package domain

import (
	"time"

	"monorepo/globalshared/gormx"
)

// JSON is a jsonb column value
type JSON = gormx.JSON

// Order is one sale in the book: one per payment of the payment service
type Order struct {
	ID          int64  `gorm:"column:id;primary_key" json:"id"`
	OrderNumber string `gorm:"column:order_number" json:"orderNumber"`
	PaymentID   string `gorm:"column:payment_id" json:"paymentId"`
	Source      string `gorm:"column:source" json:"source"`
	ReferenceID string `gorm:"column:reference_id" json:"referenceId"`
	// Where the sale is booked, from the payment metadata (merchantId / outletId / cashierId / channel)
	MerchantID string `gorm:"column:merchant_id" json:"merchantId"`
	OutletID   string `gorm:"column:outlet_id" json:"outletId"`
	CashierID  string `gorm:"column:cashier_id" json:"cashierId"`
	Channel    string `gorm:"column:channel" json:"channel"`

	Description   string `gorm:"column:description" json:"description"`
	CustomerName  string `gorm:"column:customer_name" json:"customerName"`
	CustomerEmail string `gorm:"column:customer_email" json:"customerEmail"`
	CustomerPhone string `gorm:"column:customer_phone" json:"customerPhone"`

	// Pricing breakdown computed from the items with the merchant settings of the time (snapshot below)
	Currency           string `gorm:"column:currency" json:"currency"`
	Subtotal           int64  `gorm:"column:subtotal" json:"subtotal"`
	TaxAmount          int64  `gorm:"column:tax_amount" json:"taxAmount"`
	RoundingAdjustment int64  `gorm:"column:rounding_adjustment" json:"roundingAdjustment"`
	ExpectedTotal      int64  `gorm:"column:expected_total" json:"expectedTotal"`
	// What the payment service charges: amount (asked by the caller), fee (of the method), total paid
	Amount      int64 `gorm:"column:amount" json:"amount"`
	Fee         int64 `gorm:"column:fee" json:"fee"`
	TotalAmount int64 `gorm:"column:total_amount" json:"totalAmount"`
	// AmountMismatch flags a payment amount that differs from ExpectedTotal; payment stays the truth
	AmountMismatch bool `gorm:"column:amount_mismatch" json:"amountMismatch"`

	TaxName      string  `gorm:"column:tax_name" json:"taxName"`
	TaxRate      float64 `gorm:"column:tax_rate" json:"taxRate"`
	TaxMode      string  `gorm:"column:tax_mode" json:"taxMode"`
	RoundingMode string  `gorm:"column:rounding_mode" json:"roundingMode"`
	RoundingUnit int64   `gorm:"column:rounding_unit" json:"roundingUnit"`

	PaymentStatus       string `gorm:"column:payment_status" json:"paymentStatus"`
	PaymentStatusSource string `gorm:"column:payment_status_source" json:"paymentStatusSource"`
	OrderStatus         string `gorm:"column:order_status" json:"orderStatus"`
	MethodCode          string `gorm:"column:method_code" json:"methodCode"`
	GatewayCode         string `gorm:"column:gateway_code" json:"gatewayCode"`
	TransactionID       string `gorm:"column:transaction_id" json:"transactionId"`
	AttemptCount        int    `gorm:"column:attempt_count" json:"attemptCount"`
	// ShiftID is the cash shift a cash sale landed in, RefundShiftID the one its cash refund came out of
	ShiftID       *int64 `gorm:"column:shift_id" json:"shiftId"`
	RefundShiftID *int64 `gorm:"column:refund_shift_id" json:"refundShiftId"`
	Metadata      JSON   `gorm:"column:metadata;type:jsonb" json:"metadata"`

	PlacedAt    time.Time  `gorm:"column:placed_at" json:"placedAt"`
	PaidAt      *time.Time `gorm:"column:paid_at" json:"paidAt"`
	ExpiresAt   *time.Time `gorm:"column:expires_at" json:"expiresAt"`
	ConfirmedAt *time.Time `gorm:"column:confirmed_at" json:"confirmedAt"`
	CompletedAt *time.Time `gorm:"column:completed_at" json:"completedAt"`
	CancelledAt *time.Time `gorm:"column:cancelled_at" json:"cancelledAt"`
	RefundedAt  *time.Time `gorm:"column:refunded_at" json:"refundedAt"`
	// LastEventAt is the occurredAt of the last payment event applied, to drop older non-final ones
	LastEventAt *time.Time `gorm:"column:last_event_at" json:"lastEventAt"`
	// Version increases with every change; manual updates send it back as an optimistic lock
	Version   int       `gorm:"column:version" json:"version"`
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updatedAt"`

	Items []OrderItem `gorm:"-" json:"items,omitempty"`
}

// TableName return table name of Order model
func (Order) TableName() string { return "orders" }

// OrderItem is one line of an order, written once with the order
type OrderItem struct {
	ID        int64  `gorm:"column:id;primary_key" json:"-"`
	OrderID   int64  `gorm:"column:order_id" json:"-"`
	LineNo    int    `gorm:"column:line_no" json:"lineNo"`
	Name      string `gorm:"column:name" json:"name"`
	Price     int64  `gorm:"column:price" json:"price"`
	Quantity  int    `gorm:"column:quantity" json:"quantity"`
	LineTotal int64  `gorm:"column:line_total" json:"lineTotal"`
}

// TableName return table name of OrderItem model
func (OrderItem) TableName() string { return "order_items" }

// OrderEvent is one entry of an order's timeline: a payment event consumed, or a manual change
type OrderEvent struct {
	ID            int64     `gorm:"column:id;primary_key" json:"id"`
	OrderID       int64     `gorm:"column:order_id" json:"orderId"`
	Event         string    `gorm:"column:event" json:"event"`
	Source        string    `gorm:"column:source" json:"source"`
	TransactionID string    `gorm:"column:transaction_id" json:"transactionId,omitempty"`
	FromStatus    string    `gorm:"column:from_status" json:"fromStatus,omitempty"`
	ToStatus      string    `gorm:"column:to_status" json:"toStatus,omitempty"`
	Amount        int64     `gorm:"column:amount" json:"amount"`
	Fee           int64     `gorm:"column:fee" json:"fee"`
	TotalAmount   int64     `gorm:"column:total_amount" json:"totalAmount"`
	MethodCode    string    `gorm:"column:method_code" json:"methodCode,omitempty"`
	Actor         string    `gorm:"column:actor" json:"actor,omitempty"`
	Note          string    `gorm:"column:note" json:"note,omitempty"`
	Flag          string    `gorm:"column:flag" json:"flag,omitempty"`
	Payload       JSON      `gorm:"column:payload;type:jsonb" json:"payload,omitempty"`
	Topic         string    `gorm:"column:topic" json:"topic,omitempty"`
	Partition     int32     `gorm:"column:partition" json:"partition,omitempty"`
	Offset        int64     `gorm:"column:offset" json:"offset,omitempty"`
	OccurredAt    time.Time `gorm:"column:occurred_at" json:"occurredAt"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"createdAt"`
}

// TableName return table name of OrderEvent model
func (OrderEvent) TableName() string { return "order_events" }

// Outbox is an event written in the same DB transaction as the change that caused it,
// published to Kafka afterwards
type Outbox struct {
	ID          int64      `gorm:"column:id;primary_key" json:"id"`
	EventType   string     `gorm:"column:event_type" json:"eventType"`
	Topic       string     `gorm:"column:topic" json:"topic"`
	Key         string     `gorm:"column:key" json:"key"`
	Payload     JSON       `gorm:"column:payload;type:jsonb" json:"payload"`
	Attempts    int        `gorm:"column:attempts" json:"attempts"`
	LastError   string     `gorm:"column:last_error" json:"lastError"`
	PublishedAt *time.Time `gorm:"column:published_at" json:"publishedAt"`
	CreatedAt   time.Time  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   time.Time  `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName return table name of Outbox model
func (Outbox) TableName() string { return "order_outbox" }

// NewJSONValue marshals v into a JSON value (nil becomes {})
func NewJSONValue(v any) JSON { return gormx.NewJSON(v) }
