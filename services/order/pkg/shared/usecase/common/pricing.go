package common

import (
	"math"

	shareddomain "monorepo/services/order/pkg/shared/domain"
)

// Price computes the breakdown of a basket with a merchant's tax and rounding settings. All amounts
// are integer rupiah; the tax rate is a percent with two decimals, handled as basis points.
//
//	exclusive: tax = round(subtotal × rate), pre-round = subtotal + tax
//	inclusive: tax = subtotal − round(subtotal / (1 + rate)), pre-round = subtotal
//	none:      tax = 0, pre-round = subtotal
//
// The total is pre-round rounded to the merchant's unit (nearest / up / down); the difference is
// the rounding adjustment. The ledger records the same breakdown for the items of every payment.
func Price(lines []shareddomain.PriceLine, m shareddomain.MerchantSetting) shareddomain.Breakdown {
	b := shareddomain.Breakdown{
		TaxName: m.TaxName, TaxRate: m.TaxRate, TaxMode: m.TaxMode,
		RoundingMode: m.RoundingMode, RoundingUnit: m.RoundingUnit, Currency: "IDR",
		Lines: make([]shareddomain.OrderItem, 0, len(lines)),
	}
	for i, l := range lines {
		total := l.Price * int64(l.Quantity)
		b.Lines = append(b.Lines, shareddomain.OrderItem{LineNo: i + 1, Name: l.Name, Price: l.Price, Quantity: l.Quantity, LineTotal: total})
		b.Subtotal += total
	}

	bp := int64(math.Round(m.TaxRate * 100)) // 11.00% -> 1100 basis points
	preRound := b.Subtotal
	switch {
	case bp <= 0 || m.TaxMode == shareddomain.TaxNone || m.TaxMode == "":
		b.TaxMode = shareddomain.TaxNone
	case m.TaxMode == shareddomain.TaxExclusive:
		b.TaxAmount = divRound(b.Subtotal*bp, 10000)
		preRound += b.TaxAmount
	case m.TaxMode == shareddomain.TaxInclusive:
		b.TaxAmount = b.Subtotal - divRound(b.Subtotal*10000, 10000+bp)
	}

	b.Total = Round(preRound, m.RoundingMode, m.RoundingUnit)
	b.RoundingAdjustment = b.Total - preRound
	return b
}

// Round rounds amount to a multiple of unit; mode none or a unit below 2 leaves it as is
func Round(amount int64, mode string, unit int64) int64 {
	if unit <= 1 {
		return amount
	}
	rem := amount % unit
	if rem < 0 {
		rem += unit
	}
	if rem == 0 {
		return amount
	}
	down := amount - rem
	switch mode {
	case shareddomain.RoundDown:
		return down
	case shareddomain.RoundUp:
		return down + unit
	case shareddomain.RoundNearest:
		if rem*2 >= unit {
			return down + unit
		}
		return down
	}
	return amount
}

// divRound divides rounding half away from zero
func divRound(a, b int64) int64 {
	if (a < 0) != (b < 0) {
		return -((-a + b/2) / b)
	}
	return (a + b/2) / b
}
