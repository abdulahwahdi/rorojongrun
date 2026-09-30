package domain

import (
	shareddomain "monorepo/services/order/pkg/shared/domain"

	"github.com/golangid/candi/candishared"
)

// ResponseShiftList is the shift list
type ResponseShiftList struct {
	Meta candishared.Meta         `json:"meta"`
	Data []shareddomain.CashShift `json:"data"`
}

// ResponseShift is a shift with its cash orders; an open shift shows its live totals
type ResponseShift struct {
	shareddomain.CashShift
	Orders []shareddomain.Order `json:"orders"`
}
