package repository

import (
	"context"

	shareddomain "monorepo/services/order/pkg/shared/domain"
)

// InvoiceRepository abstract interface: invoices and credit notes with their item snapshots
type InvoiceRepository interface {
	// Create inserts the invoice and its items
	Create(ctx context.Context, data *shareddomain.Invoice) error
	// Update writes status and emailed_at (the rest of an invoice never changes)
	Update(ctx context.Context, data *shareddomain.Invoice) error
	FindByID(ctx context.Context, id int64) (shareddomain.Invoice, error)
	FindByNumber(ctx context.Context, number string) (shareddomain.Invoice, error)
	// FindByOrder finds the invoice or credit note of an order
	FindByOrder(ctx context.Context, orderID int64, invoiceType string) (shareddomain.Invoice, error)
	FetchByOrder(ctx context.Context, orderID int64) ([]shareddomain.Invoice, error)
	FetchItems(ctx context.Context, invoiceIDs ...int64) ([]shareddomain.InvoiceItem, error)
	FetchAll(ctx context.Context, filter *shareddomain.FilterInvoice) ([]shareddomain.Invoice, error)
	Count(ctx context.Context, filter *shareddomain.FilterInvoice) int
	// FetchAfter reads the invoices matching filter with id > afterID in id order (keyset paging for exports)
	FetchAfter(ctx context.Context, filter *shareddomain.FilterInvoice, afterID int64, limit int) ([]shareddomain.Invoice, error)
}
