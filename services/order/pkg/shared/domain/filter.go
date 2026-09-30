package domain

import (
	"time"

	"github.com/golangid/candi/candishared"
)

// DateRange is a placed/issued date range. DateFrom and DateTo are YYYY-MM-DD (a day in the merchant's
// timezone, DateTo inclusive) or RFC3339; the usecase resolves them into From / To (To exclusive).
type DateRange struct {
	DateFrom string     `json:"dateFrom,omitempty"`
	DateTo   string     `json:"dateTo,omitempty"`
	From     *time.Time `json:"-"`
	To       *time.Time `json:"-"`
}

// FilterOrder is the order list filter, shared by the order list API and the order exports
type FilterOrder struct {
	candishared.Filter
	DateRange
	PaymentStatus string `json:"paymentStatus,omitempty"`
	OrderStatus   string `json:"orderStatus,omitempty"`
	Source        string `json:"source,omitempty"`
	MerchantID    string `json:"merchantId,omitempty"`
	OutletID      string `json:"outletId,omitempty"`
	CashierID     string `json:"cashierId,omitempty"`
	Channel       string `json:"channel,omitempty"`
	MethodCode    string `json:"methodCode,omitempty"`
	ShiftID       int64  `json:"shiftId,omitempty"`
	// AmountMismatch only orders whose payment amount differs from the priced total
	AmountMismatch bool `json:"amountMismatch,omitempty"`
}

// FilterInvoice is the invoice list filter, shared by the invoice list API and the invoice exports
type FilterInvoice struct {
	candishared.Filter
	DateRange
	Type       string `json:"type,omitempty"`
	Status     string `json:"status,omitempty"`
	MerchantID string `json:"merchantId,omitempty"`
	OutletID   string `json:"outletId,omitempty"`
	OrderID    int64  `json:"orderId,omitempty"`
}
