package common

import (
	"fmt"
	"time"

	shareddomain "monorepo/services/order/pkg/shared/domain"
)

// OrderNumberScope is the sequence scope of order numbers. Order numbers carry no merchant id, so
// the sequence belongs to the prefix: merchants sharing a prefix share it and numbers stay unique.
// They restart every day.
func OrderNumberScope(prefix string) string { return "order:" + prefix }

// OrderNumberPeriod is the day (merchant timezone) an order number counts in
func OrderNumberPeriod(m shareddomain.MerchantSetting, at time.Time) string {
	return at.In(m.Location()).Format("20060102")
}

// FormatOrderNumber renders <prefix>-YYYYMMDD-000123
func FormatOrderNumber(m shareddomain.MerchantSetting, period string, n int64) string {
	return fmt.Sprintf("%s-%s-%06d", m.OrderPrefix, period, n)
}

// InvoiceNumberScope is the sequence scope of a merchant's invoices or credit notes; they restart every month
func InvoiceNumberScope(invoiceType, merchantID string) string { return invoiceType + ":" + merchantID }

// InvoiceNumberPeriod is the month (merchant timezone) an invoice number counts in
func InvoiceNumberPeriod(m shareddomain.MerchantSetting, at time.Time) string {
	return at.In(m.Location()).Format("200601")
}

// FormatInvoiceNumber renders <prefix>/<merchantId>/YYYYMM/000123, or <prefix>/YYYYMM/000123 for the
// default merchant
func FormatInvoiceNumber(prefix, merchantID, period string, n int64) string {
	if merchantID == "" || merchantID == shareddomain.DefaultMerchant {
		return fmt.Sprintf("%s/%s/%06d", prefix, period, n)
	}
	return fmt.Sprintf("%s/%s/%s/%06d", prefix, merchantID, period, n)
}
