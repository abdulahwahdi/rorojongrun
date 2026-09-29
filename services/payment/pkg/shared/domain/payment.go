package domain

import "time"

// Payment model — one payment link/session requested by another service
type Payment struct {
	ID             string     `gorm:"column:id;primary_key" json:"id"`
	Token          string     `gorm:"column:token" json:"token"`
	Source         string     `gorm:"column:source" json:"source"`
	ReferenceID    string     `gorm:"column:reference_id" json:"referenceId"`
	Description    string     `gorm:"column:description" json:"description"`
	Amount         int64      `gorm:"column:amount" json:"amount"`
	Fee            int64      `gorm:"column:fee" json:"fee"`
	TotalAmount    int64      `gorm:"column:total_amount" json:"totalAmount"`
	Currency       string     `gorm:"column:currency" json:"currency"`
	Status         string     `gorm:"column:status" json:"status"`
	MethodCode     string     `gorm:"column:method_code" json:"methodCode"`
	Customer       JSON       `gorm:"column:customer;type:jsonb" json:"customer"`
	Items          JSON       `gorm:"column:items;type:jsonb" json:"items"`
	Metadata       JSON       `gorm:"column:metadata;type:jsonb" json:"metadata"`
	AllowedMethods JSON       `gorm:"column:allowed_methods;type:jsonb" json:"allowedMethods"`
	SuccessURL     string     `gorm:"column:success_url" json:"successUrl"`
	FailureURL     string     `gorm:"column:failure_url" json:"failureUrl"`
	ExpiresAt      time.Time  `gorm:"column:expires_at" json:"expiresAt"`
	PaidAt         *time.Time `gorm:"column:paid_at" json:"paidAt"`
	CreatedAt      time.Time  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt      time.Time  `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName return table name of Payment model
func (Payment) TableName() string { return "payments" }

// IsFinal reports whether the payment can no longer change status
func (p Payment) IsFinal() bool {
	return p.Status == PaymentPaid || p.Status == PaymentExpired || p.Status == PaymentCancelled
}

// Customer is the decoded Payment.Customer
type Customer struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
	Phone string `json:"phone,omitempty"`
}

// Item is one line of Payment.Items
type Item struct {
	Name     string `json:"name"`
	Price    int64  `json:"price"`
	Quantity int    `json:"quantity"`
}

// Transaction model — one checkout attempt of a payment
type Transaction struct {
	ID              string     `gorm:"column:id;primary_key" json:"id"`
	PaymentID       string     `gorm:"column:payment_id" json:"paymentId"`
	MethodCode      string     `gorm:"column:method_code" json:"methodCode"`
	GatewayCode     string     `gorm:"column:gateway_code" json:"gatewayCode"`
	Status          string     `gorm:"column:status" json:"status"`
	Amount          int64      `gorm:"column:amount" json:"amount"`
	Instruction     JSON       `gorm:"column:instruction;type:jsonb" json:"instruction"`
	GatewayResponse JSON       `gorm:"column:gateway_response;type:jsonb" json:"-"`
	CashCode        *string    `gorm:"column:cash_code" json:"cashCode,omitempty"`
	CashReceived    *int64     `gorm:"column:cash_received" json:"cashReceived,omitempty"`
	ConfirmedBy     *string    `gorm:"column:confirmed_by" json:"confirmedBy,omitempty"`
	ExpiresAt       time.Time  `gorm:"column:expires_at" json:"expiresAt"`
	PaidAt          *time.Time `gorm:"column:paid_at" json:"paidAt"`
	FailureReason   string     `gorm:"column:failure_reason" json:"failureReason,omitempty"`
	CreatedAt       time.Time  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt       time.Time  `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName return table name of Transaction model
func (Transaction) TableName() string { return "payment_transactions" }

// CallbackLog model — a gateway callback message consumed from Kafka
type CallbackLog struct {
	ID            int       `gorm:"column:id;primary_key" json:"id"`
	Topic         string    `gorm:"column:topic" json:"topic"`
	Partition     int32     `gorm:"column:partition" json:"partition"`
	Offset        int64     `gorm:"column:offset" json:"offset"`
	GatewayCode   string    `gorm:"column:gateway_code" json:"gatewayCode"`
	ExternalID    string    `gorm:"column:external_id" json:"externalId"`
	Headers       JSON      `gorm:"column:headers;type:jsonb" json:"headers"`
	Payload       string    `gorm:"column:payload" json:"payload"`
	Status        string    `gorm:"column:status" json:"status"`
	Error         string    `gorm:"column:error" json:"error"`
	TransactionID *string   `gorm:"column:transaction_id" json:"transactionId,omitempty"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName return table name of CallbackLog model
func (CallbackLog) TableName() string { return "payment_callback_logs" }

// Outbox model — an event written in the same DB transaction as the state
// change that caused it, published to Kafka afterwards
type Outbox struct {
	ID          int64      `gorm:"column:id;primary_key" json:"id"`
	EventType   string     `gorm:"column:event_type" json:"eventType"`
	Key         string     `gorm:"column:key" json:"key"`
	Payload     JSON       `gorm:"column:payload;type:jsonb" json:"payload"`
	Attempts    int        `gorm:"column:attempts" json:"attempts"`
	LastError   string     `gorm:"column:last_error" json:"lastError"`
	PublishedAt *time.Time `gorm:"column:published_at" json:"publishedAt"`
	CreatedAt   time.Time  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   time.Time  `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName return table name of Outbox model
func (Outbox) TableName() string { return "payment_outbox" }
