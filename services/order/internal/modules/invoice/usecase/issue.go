package usecase

import (
	"context"
	"errors"
	"fmt"

	"monorepo/globalshared/money"
	"monorepo/services/order/internal/modules/invoice/domain"
	shareddomain "monorepo/services/order/pkg/shared/domain"

	"github.com/golangid/candi/tracer"
	"gorm.io/gorm"
)

func (uc *invoiceUsecaseImpl) IssueInvoice(ctx context.Context, o *shareddomain.Order) (inv shareddomain.Invoice, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "InvoiceUsecase:IssueInvoice")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	repo := uc.repoSQL.InvoiceRepo()
	inv, err = repo.FindByOrder(ctx, o.ID, shareddomain.InvoiceTypeInvoice)
	if err == nil {
		return inv, nil // issued once per order
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return inv, err
	}
	return uc.issue(ctx, o, shareddomain.InvoiceTypeInvoice, nil)
}

func (uc *invoiceUsecaseImpl) IssueCreditNote(ctx context.Context, o *shareddomain.Order) (note shareddomain.Invoice, ok bool, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "InvoiceUsecase:IssueCreditNote")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	repo := uc.repoSQL.InvoiceRepo()
	inv, err := repo.FindByOrder(ctx, o.ID, shareddomain.InvoiceTypeInvoice)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return note, false, nil // nothing was invoiced
	}
	if err != nil {
		return note, false, err
	}
	if note, err = repo.FindByOrder(ctx, o.ID, shareddomain.InvoiceTypeCreditNote); err == nil {
		return note, true, nil // reversed once
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return note, false, err
	}
	if note, err = uc.issue(ctx, o, shareddomain.InvoiceTypeCreditNote, &inv); err != nil {
		return note, false, err
	}
	inv.Status = shareddomain.InvoiceCredited
	return note, true, repo.Update(ctx, &inv)
}

// issue numbers and writes an invoice or credit note from the order's snapshot, then queues
// order.invoice_issued, the audit trail entry and the customer email
func (uc *invoiceUsecaseImpl) issue(ctx context.Context, o *shareddomain.Order, typ string, ref *shareddomain.Invoice) (inv shareddomain.Invoice, err error) {
	m, err := uc.sharedUsecase.MerchantSettings(ctx, o.MerchantID)
	if err != nil {
		return inv, err
	}
	now := uc.now().UTC()
	number, err := uc.sharedUsecase.NextInvoiceNumber(ctx, m, typ, now)
	if err != nil {
		return inv, err
	}

	items := o.Items
	if items == nil {
		if items, err = uc.repoSQL.OrderRepo().FetchItems(ctx, o.ID); err != nil {
			return inv, err
		}
	}
	inv = shareddomain.Invoice{
		Number: number, Type: typ, OrderID: o.ID, OrderNumber: o.OrderNumber, Status: shareddomain.InvoiceIssued,
		MerchantID: o.MerchantID, OutletID: o.OutletID,
		SellerName: m.Name, SellerAddress: m.Address, SellerTaxID: m.TaxID,
		CustomerName: o.CustomerName, CustomerEmail: o.CustomerEmail, CustomerPhone: o.CustomerPhone,
		Currency: o.Currency, Subtotal: o.Subtotal, TaxName: o.TaxName, TaxRate: o.TaxRate, TaxMode: o.TaxMode,
		TaxAmount: o.TaxAmount, RoundingAdjustment: o.RoundingAdjustment, Fee: o.Fee, TotalAmount: o.TotalAmount,
		MethodCode: o.MethodCode, PaidAt: o.PaidAt, IssuedAt: now,
	}
	if ref != nil {
		inv.RefInvoiceID = &ref.ID
	}
	for _, it := range items {
		inv.Items = append(inv.Items, shareddomain.InvoiceItem{LineNo: it.LineNo, Name: it.Name, Price: it.Price, Quantity: it.Quantity, LineTotal: it.LineTotal})
	}

	emailed := false
	if inv.CustomerEmail != "" {
		inv.EmailedAt, emailed = &now, true
	}
	if err = uc.repoSQL.InvoiceRepo().Create(ctx, &inv); err != nil {
		return inv, err
	}
	if err = uc.sharedUsecase.Enqueue(ctx, shareddomain.EventInvoiceIssued, inv.Number, map[string]any{
		"event": shareddomain.EventInvoiceIssued, "occurredAt": now, "invoice": inv,
	}); err != nil {
		return inv, err
	}
	what := "Invoice"
	if typ == shareddomain.InvoiceTypeCreditNote {
		what = "Credit note"
	}
	if err = uc.sharedUsecase.LogActivity(ctx, shareddomain.OrderActivity(shareddomain.EventInvoiceIssued, o, "",
		fmt.Sprintf("%s %s issued for %s", what, inv.Number, money.FormatIDR(inv.TotalAmount)),
		map[string]any{"invoiceId": inv.ID, "invoiceNumber": inv.Number, "invoiceType": typ})); err != nil {
		return inv, err
	}
	if emailed {
		err = uc.enqueueEmail(ctx, &inv, ref)
	}
	return inv, err
}

// enqueueEmail asks the notification service to mail an invoice or credit note
func (uc *invoiceUsecaseImpl) enqueueEmail(ctx context.Context, inv *shareddomain.Invoice, ref *shareddomain.Invoice) error {
	m, err := uc.sharedUsecase.MerchantSettings(ctx, inv.MerchantID)
	if err != nil {
		return err
	}
	name := inv.CustomerName
	if name == "" {
		name = "Customer"
	}
	lines := make([]string, 0, len(inv.Items))
	for _, it := range inv.Items {
		lines = append(lines, fmt.Sprintf("%d × %s @ %s = %s", it.Quantity, it.Name, money.FormatIDR(it.Price), money.FormatIDR(it.LineTotal)))
	}
	vars := map[string]any{
		"customerName": name, "invoiceNumber": inv.Number, "orderNumber": inv.OrderNumber, "sellerName": inv.SellerName,
		"sellerAddress": inv.SellerAddress, "sellerTaxId": inv.SellerTaxID, "issuedAt": inv.IssuedAt.In(m.Location()).Format("2 Jan 2006 15:04 MST"),
		"subtotal": money.FormatIDR(inv.Subtotal), "taxName": inv.TaxName, "taxAmount": money.FormatIDR(inv.TaxAmount),
		"roundingAdjustment": money.FormatIDR(inv.RoundingAdjustment), "fee": money.FormatIDR(inv.Fee),
		"totalAmount": money.FormatIDR(inv.TotalAmount), "methodCode": inv.MethodCode, "lines": lines,
	}
	template := domain.TemplateInvoice
	if inv.Type == shareddomain.InvoiceTypeCreditNote {
		template = domain.TemplateCreditNote
		if ref != nil {
			vars["refInvoiceNumber"] = ref.Number
		}
	}
	return uc.sharedUsecase.Enqueue(ctx, shareddomain.EventNotificationEmail, inv.Number, map[string]any{
		"channel": "email", "templateCode": template, "recipient": inv.CustomerEmail, "variables": vars,
	})
}
