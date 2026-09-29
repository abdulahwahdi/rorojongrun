// Package money formats amounts for people (emails, printed invoices).
package money

import (
	"strconv"
	"strings"
)

// FormatIDR renders rupiah with dot thousand separators, e.g. 1250000 -> "Rp1.250.000"
func FormatIDR(amount int64) string {
	neg := amount < 0
	if neg {
		amount = -amount
	}
	s := strconv.FormatInt(amount, 10)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	if neg {
		return "-Rp" + b.String()
	}
	return "Rp" + b.String()
}
