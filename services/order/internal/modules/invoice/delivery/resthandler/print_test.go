package resthandler

import (
	"testing"
	"time"

	shareddomain "monorepo/services/order/pkg/shared/domain"

	"github.com/stretchr/testify/assert"
)

func Test_renderInvoice(t *testing.T) {
	inv := shareddomain.Invoice{
		Number: "INV/M1/202609/000001", Type: shareddomain.InvoiceTypeInvoice, OrderNumber: "KOP-20260929-000001",
		SellerName: "Kopi <Kita>", Subtotal: 48500, TaxName: "PPN", TaxRate: 11, TaxMode: "exclusive", TaxAmount: 5335,
		RoundingAdjustment: -35, TotalAmount: 53800, IssuedAt: time.Date(2026, 9, 29, 3, 0, 0, 0, time.UTC),
		Items: []shareddomain.InvoiceItem{{LineNo: 1, Name: "Kopi", Price: 18000, Quantity: 2, LineTotal: 36000}},
	}
	for _, format := range []string{"a4", "receipt"} {
		page, err := renderInvoice(format, inv, "", time.UTC)
		assert.NoError(t, err)
		html := string(page)
		assert.Contains(t, html, "INV/M1/202609/000001")
		assert.Contains(t, html, "Rp53.800")
		assert.Contains(t, html, "Rp36.000")
		assert.Contains(t, html, "PPN 11%")
		assert.Contains(t, html, "Kopi &lt;Kita&gt;", "seller data is escaped")
	}

	inv.Type = shareddomain.InvoiceTypeCreditNote
	page, err := renderInvoice("a4", inv, "INV/M1/202609/000001", time.UTC)
	assert.NoError(t, err)
	assert.Contains(t, string(page), "Credit Note")
	assert.Contains(t, string(page), "Reverses INV/M1/202609/000001")
}
