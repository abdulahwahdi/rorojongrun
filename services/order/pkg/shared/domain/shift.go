package domain

import "time"

// CashShift is a cashier's shift at an outlet: the cash in the drawer from opening float to the count at closing
type CashShift struct {
	ID           int64      `gorm:"column:id;primary_key" json:"id"`
	MerchantID   string     `gorm:"column:merchant_id" json:"merchantId"`
	OutletID     string     `gorm:"column:outlet_id" json:"outletId"`
	CashierID    string     `gorm:"column:cashier_id" json:"cashierId"`
	Status       string     `gorm:"column:status" json:"status"`
	OpeningFloat int64      `gorm:"column:opening_float" json:"openingFloat"`
	OpenedAt     time.Time  `gorm:"column:opened_at" json:"openedAt"`
	OpenedBy     string     `gorm:"column:opened_by" json:"openedBy"`
	ClosedAt     *time.Time `gorm:"column:closed_at" json:"closedAt"`
	ClosedBy     string     `gorm:"column:closed_by" json:"closedBy"`
	// Frozen at closing; for an open shift the API computes them live
	CashSales    int64     `gorm:"column:cash_sales" json:"cashSales"`
	CashRefunds  int64     `gorm:"column:cash_refunds" json:"cashRefunds"`
	OrderCount   int       `gorm:"column:order_count" json:"orderCount"`
	ExpectedCash int64     `gorm:"column:expected_cash" json:"expectedCash"`
	CountedCash  *int64    `gorm:"column:counted_cash" json:"countedCash"`
	Difference   *int64    `gorm:"column:difference" json:"difference"`
	Note         string    `gorm:"column:note" json:"note"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName return table name of CashShift model
func (CashShift) TableName() string { return "cash_shifts" }

// ShiftTotals is the cash movement of a shift
type ShiftTotals struct {
	CashSales   int64 `json:"cashSales"`
	CashRefunds int64 `json:"cashRefunds"`
	OrderCount  int   `json:"orderCount"`
}
