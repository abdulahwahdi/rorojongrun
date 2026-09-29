package domain

import shareddomain "monorepo/services/payment/pkg/shared/domain"

// RequestCreatePayment is what another service sends to obtain a payment link
type RequestCreatePayment struct {
	// Source is the calling service. It is taken from the token's client id when present.
	Source         string                `json:"source"`
	ReferenceID    string                `json:"referenceId"`
	Description    string                `json:"description"`
	Amount         int64                 `json:"amount"`
	Currency       string                `json:"currency"`
	Customer       shareddomain.Customer `json:"customer"`
	Items          []shareddomain.Item   `json:"items"`
	Metadata       map[string]any        `json:"metadata"`
	AllowedMethods []string              `json:"allowedMethods"`
	ExpiresInSec   int                   `json:"expiresInSec"`
	SuccessURL     string                `json:"successUrl"`
	FailureURL     string                `json:"failureUrl"`
}

// RequestSelectMethod picks the payment method of a checkout
type RequestSelectMethod struct {
	MethodCode string `json:"methodCode"`
}

// RequestConfirmCash records the cash a cashier received
type RequestConfirmCash struct {
	AmountReceived int64 `json:"amountReceived"`
}

// RequestMockCallback drives the dev-only mock gateway
type RequestMockCallback struct {
	TransactionID string `json:"transactionId"`
	Status        string `json:"status"` // paid | failed | expired
}

// CallbackMessage is one gateway callback as consumed from Kafka
type CallbackMessage struct {
	Topic     string
	Partition int32
	Offset    int64
	Headers   map[string]string // lower-case keys
	Body      []byte
}
