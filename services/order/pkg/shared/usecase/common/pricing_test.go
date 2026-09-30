package common

import (
	"testing"
	"time"

	shareddomain "monorepo/services/order/pkg/shared/domain"

	"github.com/stretchr/testify/assert"
)

func Test_Price(t *testing.T) {
	lines := []shareddomain.PriceLine{{Name: "Kopi", Price: 18000, Quantity: 2}, {Name: "Roti", Price: 12500, Quantity: 1}}

	t.Run("no tax, no rounding", func(t *testing.T) {
		b := Price(lines, shareddomain.MerchantSetting{})
		assert.EqualValues(t, 48500, b.Subtotal)
		assert.EqualValues(t, 0, b.TaxAmount)
		assert.EqualValues(t, 48500, b.Total)
		assert.Equal(t, shareddomain.TaxNone, b.TaxMode)
		assert.Len(t, b.Lines, 2)
		assert.EqualValues(t, 36000, b.Lines[0].LineTotal)
		assert.Equal(t, 2, b.Lines[1].LineNo)
	})

	t.Run("exclusive PPN 11% rounded to the nearest 100", func(t *testing.T) {
		m := shareddomain.MerchantSetting{TaxName: "PPN", TaxRate: 11, TaxMode: shareddomain.TaxExclusive, RoundingMode: shareddomain.RoundNearest, RoundingUnit: 100}
		b := Price(lines, m)
		assert.EqualValues(t, 5335, b.TaxAmount) // 48500 × 11% = 5335
		assert.EqualValues(t, 53800, b.Total)    // 53835 -> 53800
		assert.EqualValues(t, -35, b.RoundingAdjustment)
	})

	t.Run("inclusive tax is extracted, the total does not change", func(t *testing.T) {
		m := shareddomain.MerchantSetting{TaxRate: 11, TaxMode: shareddomain.TaxInclusive}
		b := Price([]shareddomain.PriceLine{{Name: "x", Price: 111000, Quantity: 1}}, m)
		assert.EqualValues(t, 11000, b.TaxAmount)
		assert.EqualValues(t, 111000, b.Total)
		assert.EqualValues(t, 0, b.RoundingAdjustment)
	})

	t.Run("fractional rates are exact to two decimals", func(t *testing.T) {
		m := shareddomain.MerchantSetting{TaxRate: 12.5, TaxMode: shareddomain.TaxExclusive}
		b := Price([]shareddomain.PriceLine{{Name: "x", Price: 10001, Quantity: 1}}, m)
		assert.EqualValues(t, 1250, b.TaxAmount) // 1250.125 -> 1250
	})
}

func Test_Round(t *testing.T) {
	cases := []struct {
		amount int64
		mode   string
		unit   int64
		want   int64
	}{
		{12349, shareddomain.RoundNearest, 100, 12300},
		{12350, shareddomain.RoundNearest, 100, 12400},
		{12301, shareddomain.RoundUp, 100, 12400},
		{12399, shareddomain.RoundDown, 100, 12300},
		{12300, shareddomain.RoundUp, 100, 12300},
		{12345, shareddomain.RoundNone, 100, 12345},
		{12345, shareddomain.RoundNearest, 1, 12345},
		{12345, shareddomain.RoundUp, 500, 12500},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, Round(c.amount, c.mode, c.unit), "%d %s %d", c.amount, c.mode, c.unit)
	}
}

func Test_Numbering(t *testing.T) {
	m := shareddomain.MerchantSetting{MerchantID: "M1", OrderPrefix: "KOP", Timezone: "Asia/Jakarta"}
	// 20:30 UTC is already the next day in Jakarta
	at := time.Date(2026, 9, 29, 20, 30, 0, 0, time.UTC)
	assert.Equal(t, "20260930", OrderNumberPeriod(m, at))
	assert.Equal(t, "KOP-20260930-000042", FormatOrderNumber(m, "20260930", 42))
	assert.Equal(t, "202609", InvoiceNumberPeriod(m, time.Date(2026, 9, 30, 16, 0, 0, 0, time.UTC)))
	assert.Equal(t, "202610", InvoiceNumberPeriod(m, time.Date(2026, 9, 30, 17, 0, 0, 0, time.UTC)))
	assert.Equal(t, "INV/M1/202609/000007", FormatInvoiceNumber("INV", "M1", "202609", 7))
	assert.Equal(t, "CN/202609/000001", FormatInvoiceNumber("CN", "*", "202609", 1))
	assert.Equal(t, "order:KOP", OrderNumberScope("KOP"))
	assert.Equal(t, "credit_note:M1", InvoiceNumberScope(shareddomain.InvoiceTypeCreditNote, "M1"))
}
