package domain

import "time"

// Method model — a payment method shown on the checkout page
type Method struct {
	ID             int       `gorm:"column:id;primary_key" json:"id"`
	Code           string    `gorm:"column:code;type:varchar(50)" json:"code"`
	Name           string    `gorm:"column:name;type:varchar(100)" json:"name"`
	Type           string    `gorm:"column:type;type:varchar(30)" json:"type"`
	GatewayCode    *string   `gorm:"column:gateway_code;type:varchar(50)" json:"gatewayCode"`
	GatewayChannel string    `gorm:"column:gateway_channel;type:varchar(50)" json:"gatewayChannel"`
	IsEnabled      bool      `gorm:"column:is_enabled" json:"isEnabled"`
	IconURL        string    `gorm:"column:icon_url" json:"iconUrl"`
	SortOrder      int       `gorm:"column:sort_order" json:"sortOrder"`
	MinAmount      int64     `gorm:"column:min_amount" json:"minAmount"`
	MaxAmount      int64     `gorm:"column:max_amount" json:"maxAmount"`
	FeeFlat        int64     `gorm:"column:fee_flat" json:"feeFlat"`
	FeePercent     float64   `gorm:"column:fee_percent" json:"feePercent"`
	Instructions   string    `gorm:"column:instructions" json:"instructions"`
	CreatedAt      time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt      time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName return table name of Method model
func (Method) TableName() string { return "payment_methods" }

// IsCash reports whether the method is settled offline
func (m Method) IsCash() bool { return m.Type == MethodCash }

// CalculateFee returns flat + percent fee for amount, rounded up to the smallest unit
func (m Method) CalculateFee(amount int64) int64 {
	fee := m.FeeFlat
	if m.FeePercent > 0 {
		pct := int64(float64(amount)*m.FeePercent/100 + 0.999999)
		fee += pct
	}
	return fee
}

// AmountAllowed checks the method's min/max range (max 0 = unlimited)
func (m Method) AmountAllowed(amount int64) bool {
	return amount >= m.MinAmount && (m.MaxAmount == 0 || amount <= m.MaxAmount)
}
