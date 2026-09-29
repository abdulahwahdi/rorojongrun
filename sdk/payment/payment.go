package payment

import "context"

// Payment client abstract interface. Other services use it to obtain a payment link; the
// customer then completes every checkout step (pick a method, pay) in the payment service.
// The outcome arrives as Kafka events: payment.checkout_started, payment.completed,
// payment.expired and payment.cancelled (topic names are configurable in the payment service DB).
type Payment interface {
	// CreatePayment issues a payment link. Idempotent per (Source, ReferenceID) while the payment is open.
	CreatePayment(ctx context.Context, req CreatePaymentRequest) (CreatePaymentResponse, error)
	GetPayment(ctx context.Context, id string) (PaymentResponse, error)
	// CancelPayment cancels an open payment; cancelling twice is not an error.
	CancelPayment(ctx context.Context, id string) (PaymentResponse, error)
}

// Customer of a payment; Email is where checkout / paid emails go (optional)
type Customer struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
	Phone string `json:"phone,omitempty"`
}

// Item is one line of a payment
type Item struct {
	Name     string `json:"name"`
	Price    int64  `json:"price"`
	Quantity int    `json:"quantity"`
}

// CreatePaymentRequest is the payload for CreatePayment. Kept as a local copy of the service's
// shape — sdk/ clients must not import a service's internal/ packages.
type CreatePaymentRequest struct {
	Source         string         `json:"source"`      // the calling service, e.g. "order" (REST: taken from a service token)
	ReferenceID    string         `json:"referenceId"` // the caller's own id, e.g. the order id
	Description    string         `json:"description,omitempty"`
	Amount         int64          `json:"amount"` // IDR
	Customer       Customer       `json:"customer,omitempty"`
	Items          []Item         `json:"items,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"` // echoed back in payment events
	AllowedMethods []string       `json:"allowedMethods,omitempty"`
	ExpiresInSec   int            `json:"expiresInSec,omitempty"` // 0 = service default
	SuccessURL     string         `json:"successUrl,omitempty"`
	FailureURL     string         `json:"failureUrl,omitempty"`
}

// CreatePaymentResponse carries the link to hand to the customer
type CreatePaymentResponse struct {
	PaymentID   string `json:"paymentId"`
	Token       string `json:"token"`
	PaymentURL  string `json:"paymentUrl"`
	Status      string `json:"status"`
	ReferenceID string `json:"referenceId"`
	Amount      int64  `json:"amount"`
	Currency    string `json:"currency"`
	ExpiresAt   string `json:"expiresAt"`
}

// PaymentResponse mirrors the payment model of the payment service
type PaymentResponse struct {
	ID          string `json:"id"`
	Source      string `json:"source"`
	ReferenceID string `json:"referenceId"`
	Description string `json:"description"`
	Amount      int64  `json:"amount"`
	Fee         int64  `json:"fee"`
	TotalAmount int64  `json:"totalAmount"`
	Currency    string `json:"currency"`
	Status      string `json:"status"` // pending | processing | paid | expired | cancelled
	MethodCode  string `json:"methodCode"`
	ExpiresAt   string `json:"expiresAt"`
	PaidAt      string `json:"paidAt,omitempty"`
	CreatedAt   string `json:"createdAt"`
}
