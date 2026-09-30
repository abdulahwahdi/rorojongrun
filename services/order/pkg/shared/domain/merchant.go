package domain

import "time"

// MerchantSetting is the per-merchant configuration of numbering, tax, rounding and the seller data
// printed on invoices. The row DefaultMerchant ("*") applies to merchants without their own row.
type MerchantSetting struct {
	MerchantID       string    `gorm:"column:merchant_id;primary_key" json:"merchantId"`
	Name             string    `gorm:"column:name" json:"name"`
	Address          string    `gorm:"column:address" json:"address"`
	TaxID            string    `gorm:"column:tax_id" json:"taxId"`
	OrderPrefix      string    `gorm:"column:order_prefix" json:"orderPrefix"`
	InvoicePrefix    string    `gorm:"column:invoice_prefix" json:"invoicePrefix"`
	CreditNotePrefix string    `gorm:"column:credit_note_prefix" json:"creditNotePrefix"`
	TaxName          string    `gorm:"column:tax_name" json:"taxName"`
	TaxRate          float64   `gorm:"column:tax_rate" json:"taxRate"`
	TaxMode          string    `gorm:"column:tax_mode" json:"taxMode"`
	RoundingMode     string    `gorm:"column:rounding_mode" json:"roundingMode"`
	RoundingUnit     int64     `gorm:"column:rounding_unit" json:"roundingUnit"`
	Timezone         string    `gorm:"column:timezone" json:"timezone"`
	CreatedAt        time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt        time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName return table name of MerchantSetting model
func (MerchantSetting) TableName() string { return "merchant_settings" }

// Location is the merchant's timezone, UTC when it cannot be loaded
func (m MerchantSetting) Location() *time.Location {
	if loc, err := time.LoadLocation(m.Timezone); err == nil && m.Timezone != "" {
		return loc
	}
	return time.UTC
}

// NumberSequence is the last number handed out in a numbering scope and period
type NumberSequence struct {
	Scope  string `gorm:"column:scope;primary_key"`
	Period string `gorm:"column:period;primary_key"`
	LastNo int64  `gorm:"column:last_no"`
}

// TableName return table name of NumberSequence model
func (NumberSequence) TableName() string { return "number_sequences" }
