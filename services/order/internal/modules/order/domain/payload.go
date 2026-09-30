package domain

import (
	"fmt"
	"strconv"
	"time"
)

// PaymentEvent is the payload of the payment service's payment.* events
// (services/payment/internal/modules/payment/domain.PaymentEvent). Every event is a full snapshot.
type PaymentEvent struct {
	Event         string         `json:"event"`
	PaymentID     string         `json:"paymentId"`
	TransactionID string         `json:"transactionId"`
	Source        string         `json:"source"`
	ReferenceID   string         `json:"referenceId"`
	Description   string         `json:"description"`
	Status        string         `json:"status"`
	Amount        int64          `json:"amount"`
	Fee           int64          `json:"fee"`
	TotalAmount   int64          `json:"totalAmount"`
	Currency      string         `json:"currency"`
	MethodCode    string         `json:"methodCode"`
	GatewayCode   string         `json:"gatewayCode"`
	PaidAt        *time.Time     `json:"paidAt"`
	ExpiresAt     *time.Time     `json:"expiresAt"`
	Metadata      map[string]any `json:"metadata"`
	Customer      struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Phone string `json:"phone"`
	} `json:"customer"`
	Items []struct {
		Name     string `json:"name"`
		Price    int64  `json:"price"`
		Quantity int    `json:"quantity"`
	} `json:"items"`
	CreatedAt  time.Time `json:"createdAt"`
	OccurredAt time.Time `json:"occurredAt"`
}

// Meta returns a metadata value as a string ("" when missing or not a scalar)
func (e *PaymentEvent) Meta(key string) string {
	switch v := e.Metadata[key].(type) {
	case string:
		return v
	case float64, bool, int, int64:
		return fmtScalar(v)
	}
	return ""
}

// KafkaSource locates a consumed message, recorded on the timeline
type KafkaSource struct {
	Topic     string
	Partition int32
	Offset    int64
}

func fmtScalar(v any) string {
	switch x := v.(type) {
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	default:
		return fmt.Sprint(x)
	}
}
