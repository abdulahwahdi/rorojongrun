package domain

import "github.com/golangid/candi/candishared"

// FilterShift model
type FilterShift struct {
	candishared.Filter
	MerchantID string `json:"merchantId,omitempty"`
	OutletID   string `json:"outletId,omitempty"`
	CashierID  string `json:"cashierId,omitempty"`
	Status     string `json:"status,omitempty"`
}
