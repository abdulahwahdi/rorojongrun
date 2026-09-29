package domain

import (
	"time"

	shareddomain "monorepo/services/payment/pkg/shared/domain"

	"github.com/golangid/candi/candishared"
)

// ResponseCreatePayment is returned to the calling service
type ResponseCreatePayment struct {
	PaymentID   string `json:"paymentId"`
	Token       string `json:"token"`
	PaymentURL  string `json:"paymentUrl"`
	Status      string `json:"status"`
	ReferenceID string `json:"referenceId"`
	Amount      int64  `json:"amount"`
	Currency    string `json:"currency"`
	ExpiresAt   string `json:"expiresAt"`
}

// ResponsePayment is the caller/admin view of a payment
type ResponsePayment struct {
	shareddomain.Payment
	Transactions []shareddomain.Transaction `json:"transactions"`
}

// ResponsePaymentList model
type ResponsePaymentList struct {
	Meta candishared.Meta       `json:"meta"`
	Data []shareddomain.Payment `json:"data"`
}

// ResponseCheckoutMethod is one selectable method on the checkout page
type ResponseCheckoutMethod struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	IconURL      string `json:"iconUrl"`
	Fee          int64  `json:"fee"`
	TotalAmount  int64  `json:"totalAmount"`
	Instructions string `json:"instructions,omitempty"`
}

// ResponseCheckoutTransaction is the current payment attempt as the customer sees it
type ResponseCheckoutTransaction struct {
	ID          string         `json:"id"`
	Status      string         `json:"status"`
	MethodCode  string         `json:"methodCode"`
	Amount      int64          `json:"amount"`
	Instruction map[string]any `json:"instruction"`
	ExpiresAt   string         `json:"expiresAt"`
}

// ResponseCheckout is the public checkout view; the token is the credential
type ResponseCheckout struct {
	PaymentID   string                       `json:"paymentId"`
	Status      string                       `json:"status"`
	ReferenceID string                       `json:"referenceId"`
	Description string                       `json:"description"`
	Amount      int64                        `json:"amount"`
	Fee         int64                        `json:"fee"`
	TotalAmount int64                        `json:"totalAmount"`
	Currency    string                       `json:"currency"`
	Items       []shareddomain.Item          `json:"items"`
	MethodCode  string                       `json:"methodCode"`
	ExpiresAt   string                       `json:"expiresAt"`
	PaidAt      string                       `json:"paidAt,omitempty"`
	SuccessURL  string                       `json:"successUrl,omitempty"`
	FailureURL  string                       `json:"failureUrl,omitempty"`
	Transaction *ResponseCheckoutTransaction `json:"transaction,omitempty"`
}

// ResponseCash is what a cashier sees before confirming
type ResponseCash struct {
	CashCode    string `json:"cashCode"`
	PaymentID   string `json:"paymentId"`
	ReferenceID string `json:"referenceId"`
	Source      string `json:"source"`
	Status      string `json:"status"`
	TotalAmount int64  `json:"totalAmount"`
	Currency    string `json:"currency"`
	ExpiresAt   string `json:"expiresAt"`
}

// ResponseCashConfirm is returned after a cash payment is confirmed
type ResponseCashConfirm struct {
	ResponseCash
	AmountReceived int64 `json:"amountReceived"`
	Change         int64 `json:"change"`
}

// ResponseCallbackLogList model
type ResponseCallbackLogList struct {
	Meta candishared.Meta           `json:"meta"`
	Data []shareddomain.CallbackLog `json:"data"`
}

// CallbackOutcome is what HandleCallback decided
type CallbackOutcome struct {
	Status        string // shareddomain.Callback* values
	TransactionID string
	Message       string
}

// PaymentEvent is the payload of the outbound payment.* events
type PaymentEvent struct {
	Event     string `json:"event"`
	PaymentID string `json:"paymentId"`
	// TransactionID is set on checkout/completion events: the attempt the event is about
	TransactionID string         `json:"transactionId,omitempty"`
	Source        string         `json:"source"`
	ReferenceID   string         `json:"referenceId"`
	Status        string         `json:"status"`
	Amount        int64          `json:"amount"`
	Fee           int64          `json:"fee"`
	TotalAmount   int64          `json:"totalAmount"`
	Currency      string         `json:"currency"`
	MethodCode    string         `json:"methodCode,omitempty"`
	GatewayCode   string         `json:"gatewayCode,omitempty"`
	PaidAt        *time.Time     `json:"paidAt,omitempty"`
	ExpiresAt     time.Time      `json:"expiresAt"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

// NotificationRequest mirrors notification.requested's payload (sdk/notification.SendNotificationRequest)
type NotificationRequest struct {
	Channel      string         `json:"channel"`
	TemplateCode string         `json:"templateCode"`
	Recipient    string         `json:"recipient"`
	Variables    map[string]any `json:"variables,omitempty"`
}
