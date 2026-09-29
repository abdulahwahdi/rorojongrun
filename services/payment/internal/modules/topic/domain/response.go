package domain

import (
	"github.com/golangid/candi/candishared"

	shareddomain "monorepo/services/payment/pkg/shared/domain"
)

// ResponseTopicList model
type ResponseTopicList struct {
	Meta candishared.Meta     `json:"meta"`
	Data []shareddomain.Topic `json:"data"`
}
