package usecase

import (
	"io"
	"strconv"
	"time"

	shareddomain "monorepo/services/order/pkg/shared/domain"
)

var orderHeader = []string{
	"order_number", "placed_at", "merchant_id", "outlet_id", "cashier_id", "channel", "source", "reference_id",
	"customer_name", "customer_email", "customer_phone", "payment_status", "order_status", "method_code",
	"subtotal", "tax_name", "tax_rate", "tax_mode", "tax_amount", "rounding_adjustment", "expected_total",
	"amount", "fee", "total_amount", "amount_mismatch", "currency", "paid_at", "completed_at", "cancelled_at",
	"refunded_at", "shift_id",
}

var orderLineHeader = []string{
	"order_number", "placed_at", "merchant_id", "outlet_id", "payment_status", "order_status",
	"line_no", "item_name", "price", "quantity", "line_total",
}

var invoiceHeader = []string{
	"number", "type", "status", "order_number", "issued_at", "merchant_id", "outlet_id", "seller_name",
	"seller_tax_id", "customer_name", "customer_email", "currency", "subtotal", "tax_name", "tax_rate",
	"tax_mode", "tax_amount", "rounding_adjustment", "fee", "total_amount", "method_code", "paid_at",
}

func ts(t *time.Time, loc *time.Location) string {
	if t == nil {
		return ""
	}
	return t.In(loc).Format("2006-01-02 15:04:05")
}

func i64(n int64) string { return strconv.FormatInt(n, 10) }

func rate(r float64) string { return strconv.FormatFloat(r, 'f', 2, 64) }

func orderRow(o *shareddomain.Order, loc *time.Location) []string {
	shift := ""
	if o.ShiftID != nil {
		shift = i64(*o.ShiftID)
	}
	return []string{
		o.OrderNumber, ts(&o.PlacedAt, loc), o.MerchantID, o.OutletID, o.CashierID, o.Channel, o.Source, o.ReferenceID,
		o.CustomerName, o.CustomerEmail, o.CustomerPhone, o.PaymentStatus, o.OrderStatus, o.MethodCode,
		i64(o.Subtotal), o.TaxName, rate(o.TaxRate), o.TaxMode, i64(o.TaxAmount), i64(o.RoundingAdjustment), i64(o.ExpectedTotal),
		i64(o.Amount), i64(o.Fee), i64(o.TotalAmount), strconv.FormatBool(o.AmountMismatch), o.Currency,
		ts(o.PaidAt, loc), ts(o.CompletedAt, loc), ts(o.CancelledAt, loc), ts(o.RefundedAt, loc), shift,
	}
}

func orderLineRow(o *shareddomain.Order, it *shareddomain.OrderItem, loc *time.Location) []string {
	return []string{
		o.OrderNumber, ts(&o.PlacedAt, loc), o.MerchantID, o.OutletID, o.PaymentStatus, o.OrderStatus,
		strconv.Itoa(it.LineNo), it.Name, i64(it.Price), strconv.Itoa(it.Quantity), i64(it.LineTotal),
	}
}

func invoiceRow(inv *shareddomain.Invoice, loc *time.Location) []string {
	return []string{
		inv.Number, inv.Type, inv.Status, inv.OrderNumber, ts(&inv.IssuedAt, loc), inv.MerchantID, inv.OutletID, inv.SellerName,
		inv.SellerTaxID, inv.CustomerName, inv.CustomerEmail, inv.Currency, i64(inv.Subtotal), inv.TaxName, rate(inv.TaxRate),
		inv.TaxMode, i64(inv.TaxAmount), i64(inv.RoundingAdjustment), i64(inv.Fee), i64(inv.TotalAmount), inv.MethodCode, ts(inv.PaidAt, loc),
	}
}

// countingWriter counts the bytes of the file it writes
type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}
