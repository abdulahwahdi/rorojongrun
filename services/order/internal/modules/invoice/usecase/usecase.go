package usecase

import (
	"context"
	"time"

	"monorepo/services/order/internal/modules/invoice/domain"
	shareddomain "monorepo/services/order/pkg/shared/domain"
	"monorepo/services/order/pkg/shared/repository"
	"monorepo/services/order/pkg/shared/usecase/common"

	"github.com/golangid/candi/codebase/factory/dependency"
)

// InvoiceUsecase abstraction: invoices issued when an order is paid, credit notes when it is refunded
type InvoiceUsecase interface {
	GetAllInvoices(ctx context.Context, filter *shareddomain.FilterInvoice) (res domain.ResponseInvoiceList, err error)
	GetInvoice(ctx context.Context, id int64) (res shareddomain.Invoice, err error)
	GetInvoiceByNumber(ctx context.Context, number string) (res shareddomain.Invoice, err error)
	// ResendInvoice emails an invoice or credit note to the customer again
	ResendInvoice(ctx context.Context, id int64, actor string) (res shareddomain.Invoice, err error)

	// shared with the other modules (common.Usecase)
	IssueInvoice(ctx context.Context, order *shareddomain.Order) (shareddomain.Invoice, error)
	IssueCreditNote(ctx context.Context, order *shareddomain.Order) (note shareddomain.Invoice, ok bool, err error)
	InvoicesOfOrder(ctx context.Context, orderID int64) ([]shareddomain.Invoice, error)
}

type invoiceUsecaseImpl struct {
	deps          dependency.Dependency
	sharedUsecase common.Usecase
	repoSQL       repository.RepoSQL
	now           func() time.Time
}

// NewInvoiceUsecase usecase impl constructor
func NewInvoiceUsecase(deps dependency.Dependency) (InvoiceUsecase, func(sharedUsecase common.Usecase)) {
	uc := &invoiceUsecaseImpl{deps: deps, repoSQL: repository.GetSharedRepoSQL(), now: time.Now}
	return uc, func(sharedUsecase common.Usecase) {
		uc.sharedUsecase = sharedUsecase
	}
}
