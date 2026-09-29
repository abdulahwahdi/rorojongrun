package domain

import (
	shareddomain "monorepo/services/order/pkg/shared/domain"

	"github.com/golangid/candi/candishared"
)

// ResponseInvoiceList is the invoice list
type ResponseInvoiceList struct {
	Meta candishared.Meta       `json:"meta"`
	Data []shareddomain.Invoice `json:"data"`
}
