package domain

import (
	"github.com/golangid/candi/candishared"

	shareddomain "monorepo/services/payment/pkg/shared/domain"
)

// ResponseMethodList model
type ResponseMethodList struct {
	Meta candishared.Meta      `json:"meta"`
	Data []shareddomain.Method `json:"data"`
}
