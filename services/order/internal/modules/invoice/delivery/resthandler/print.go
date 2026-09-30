package resthandler

import (
	"bytes"
	"html/template"
	"time"

	"monorepo/globalshared/money"
	"monorepo/services/order/api"
	shareddomain "monorepo/services/order/pkg/shared/domain"
)

var printTemplates = template.Must(template.New("invoice").Funcs(template.FuncMap{
	"idr": money.FormatIDR,
}).ParseFS(api.Templates, "templates/*.html"))

// printView is what the invoice templates render
type printView struct {
	Invoice      shareddomain.Invoice
	Title        string
	RefNumber    string
	IssuedAt     string
	PaidAt       string
	IsCreditNote bool
}

// renderInvoice renders the printable HTML of an invoice or credit note, format a4 or receipt
func renderInvoice(format string, inv shareddomain.Invoice, refNumber string, loc *time.Location) ([]byte, error) {
	v := printView{Invoice: inv, Title: "Invoice", RefNumber: refNumber, IssuedAt: inv.IssuedAt.In(loc).Format("2 Jan 2006 15:04 MST")}
	if inv.Type == shareddomain.InvoiceTypeCreditNote {
		v.Title, v.IsCreditNote = "Credit Note", true
	}
	if inv.PaidAt != nil {
		v.PaidAt = inv.PaidAt.In(loc).Format("2 Jan 2006 15:04 MST")
	}
	var buf bytes.Buffer
	err := printTemplates.ExecuteTemplate(&buf, "invoice_"+format+".html", v)
	return buf.Bytes(), err
}
