package domain

import (
	shareddomain "monorepo/services/order/pkg/shared/domain"

	"github.com/golangid/candi/candishared"
)

// ResponseExportList is the export job list
type ResponseExportList struct {
	Meta candishared.Meta         `json:"meta"`
	Data []shareddomain.ExportJob `json:"data"`
}
