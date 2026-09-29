package usecase

import (
	"testing"
	"time"

	shareddomain "monorepo/services/order/pkg/shared/domain"

	"github.com/stretchr/testify/assert"
)

func Test_CSVRows(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	placed := time.Date(2026, 9, 29, 3, 0, 0, 0, time.UTC)
	shift := int64(9)
	o := shareddomain.Order{OrderNumber: "KOP-20260929-000001", PlacedAt: placed, MerchantID: "M1", TaxRate: 11, TotalAmount: 53800, ShiftID: &shift, AmountMismatch: true}

	row := orderRow(&o, loc)
	assert.Len(t, row, len(orderHeader))
	assert.Equal(t, "2026-09-29 10:00:00", row[1], "times are in the merchant's timezone")
	assert.Equal(t, "11.00", row[16])
	assert.Equal(t, "53800", row[23])
	assert.Equal(t, "true", row[24])
	assert.Equal(t, "9", row[30])

	line := orderLineRow(&o, &shareddomain.OrderItem{LineNo: 1, Name: "Kopi", Price: 18000, Quantity: 2, LineTotal: 36000}, loc)
	assert.Len(t, line, len(orderLineHeader))
	assert.Equal(t, []string{"1", "Kopi", "18000", "2", "36000"}, line[6:])

	inv := shareddomain.Invoice{Number: "INV/M1/202609/000001", IssuedAt: placed}
	assert.Len(t, invoiceRow(&inv, loc), len(invoiceHeader))
}
