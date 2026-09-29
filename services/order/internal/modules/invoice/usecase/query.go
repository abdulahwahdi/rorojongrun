package usecase

import (
	"context"
	"errors"
	"fmt"

	"monorepo/globalshared/rest"
	"monorepo/services/order/internal/modules/invoice/domain"
	shareddomain "monorepo/services/order/pkg/shared/domain"

	"github.com/golangid/candi/candishared"
	"github.com/golangid/candi/tracer"
	"gorm.io/gorm"
)

func (uc *invoiceUsecaseImpl) GetAllInvoices(ctx context.Context, filter *shareddomain.FilterInvoice) (res domain.ResponseInvoiceList, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "InvoiceUsecase:GetAllInvoices")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	repo := uc.repoSQL.InvoiceRepo()
	if res.Data, err = repo.FetchAll(ctx, filter); err != nil {
		return
	}
	if res.Data == nil {
		res.Data = []shareddomain.Invoice{}
	}
	res.Meta = candishared.NewMeta(filter.Page, filter.Limit, repo.Count(ctx, filter))
	return
}

func (uc *invoiceUsecaseImpl) withItems(ctx context.Context, inv shareddomain.Invoice, err error) (shareddomain.Invoice, error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return inv, rest.NewNotFound("invoice not found")
	}
	if err != nil {
		return inv, err
	}
	inv.Items, err = uc.repoSQL.InvoiceRepo().FetchItems(ctx, inv.ID)
	if inv.Items == nil {
		inv.Items = []shareddomain.InvoiceItem{}
	}
	return inv, err
}

func (uc *invoiceUsecaseImpl) GetInvoice(ctx context.Context, id int64) (shareddomain.Invoice, error) {
	inv, err := uc.repoSQL.InvoiceRepo().FindByID(ctx, id)
	return uc.withItems(ctx, inv, err)
}

func (uc *invoiceUsecaseImpl) GetInvoiceByNumber(ctx context.Context, number string) (shareddomain.Invoice, error) {
	inv, err := uc.repoSQL.InvoiceRepo().FindByNumber(ctx, number)
	return uc.withItems(ctx, inv, err)
}

func (uc *invoiceUsecaseImpl) InvoicesOfOrder(ctx context.Context, orderID int64) ([]shareddomain.Invoice, error) {
	return uc.repoSQL.InvoiceRepo().FetchByOrder(ctx, orderID)
}

func (uc *invoiceUsecaseImpl) ResendInvoice(ctx context.Context, id int64, actor string) (inv shareddomain.Invoice, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "InvoiceUsecase:ResendInvoice")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = uc.repoSQL.WithTransaction(ctx, func(ctx context.Context) error {
		if inv, err = uc.GetInvoice(ctx, id); err != nil {
			return err
		}
		if inv.CustomerEmail == "" {
			return rest.NewConflict("the customer has no email address")
		}
		var ref *shareddomain.Invoice
		if inv.RefInvoiceID != nil {
			r, err := uc.repoSQL.InvoiceRepo().FindByID(ctx, *inv.RefInvoiceID)
			if err != nil {
				return err
			}
			ref = &r
		}
		if err := uc.enqueueEmail(ctx, &inv, ref); err != nil {
			return err
		}
		now := uc.now().UTC()
		inv.EmailedAt = &now
		if err := uc.repoSQL.InvoiceRepo().Update(ctx, &inv); err != nil {
			return err
		}
		order, err := uc.repoSQL.OrderRepo().FindByID(ctx, inv.OrderID)
		if err != nil {
			return err
		}
		return uc.sharedUsecase.LogActivity(ctx, shareddomain.OrderActivity("order.invoice_resent", &order, actor,
			fmt.Sprintf("%s emailed again to %s", inv.Number, inv.CustomerEmail), map[string]any{"invoiceId": inv.ID}))
	})
	if err == nil {
		uc.sharedUsecase.KickOutbox()
	}
	return inv, err
}
