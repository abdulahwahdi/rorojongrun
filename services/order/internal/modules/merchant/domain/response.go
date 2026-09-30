package domain

import (
	shareddomain "monorepo/services/order/pkg/shared/domain"

	"github.com/golangid/candi/candishared"
)

// ResponseMerchantList is the merchant settings list
type ResponseMerchantList struct {
	Meta candishared.Meta               `json:"meta"`
	Data []shareddomain.MerchantSetting `json:"data"`
}
