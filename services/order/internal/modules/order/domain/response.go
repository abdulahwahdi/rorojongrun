package domain

import (
	shareddomain "monorepo/services/order/pkg/shared/domain"

	"github.com/golangid/candi/candishared"
)

// ResponseOrderList is the order list
type ResponseOrderList struct {
	Meta candishared.Meta     `json:"meta"`
	Data []shareddomain.Order `json:"data"`
}

// ResponseOrder is an order with its items, timeline and invoices
type ResponseOrder struct {
	shareddomain.Order
	Events   []shareddomain.OrderEvent `json:"events"`
	Invoices []shareddomain.Invoice    `json:"invoices"`
	// AllowedStatuses are the order statuses staff can move this order to now
	AllowedStatuses []string `json:"allowedStatuses"`
}

// StatusCount is the number of orders in a status
type StatusCount struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

// SummaryTotals are the bookkeeping totals of a report. Money columns count paid orders only.
type SummaryTotals struct {
	OrderCount         int64         `json:"orderCount"`
	PaidCount          int64         `json:"paidCount"`
	Subtotal           int64         `json:"subtotal"`
	TaxAmount          int64         `json:"taxAmount"`
	RoundingAdjustment int64         `json:"roundingAdjustment"`
	Gross              int64         `json:"gross"`
	Fee                int64         `json:"fee"`
	TotalAmount        int64         `json:"totalAmount"`
	RefundedCount      int64         `json:"refundedCount"`
	RefundedAmount     int64         `json:"refundedAmount"`
	Net                int64         `json:"net" gorm:"-"`
	ByPaymentStatus    []StatusCount `json:"byPaymentStatus" gorm:"-"`
	ByOrderStatus      []StatusCount `json:"byOrderStatus" gorm:"-"`
}

// SummaryGroup is one row of a grouped report
type SummaryGroup struct {
	Key                string `json:"key"`
	OrderCount         int64  `json:"orderCount"`
	PaidCount          int64  `json:"paidCount"`
	Subtotal           int64  `json:"subtotal"`
	TaxAmount          int64  `json:"taxAmount"`
	RoundingAdjustment int64  `json:"roundingAdjustment"`
	Gross              int64  `json:"gross"`
	Fee                int64  `json:"fee"`
	TotalAmount        int64  `json:"totalAmount"`
	RefundedCount      int64  `json:"refundedCount"`
	RefundedAmount     int64  `json:"refundedAmount"`
	Net                int64  `json:"net" gorm:"-"`
}

// ResponseSummary is the bookkeeping report
type ResponseSummary struct {
	From     string         `json:"from,omitempty"`
	To       string         `json:"to,omitempty"`
	Timezone string         `json:"timezone"`
	GroupBy  string         `json:"groupBy,omitempty"`
	Totals   SummaryTotals  `json:"totals"`
	Groups   []SummaryGroup `json:"groups"`
}
