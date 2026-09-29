package domain

import "time"

// Invoice is the sales invoice issued when an order's payment is paid, or the credit note that
// reverses it on a refund. Never changed after it is issued, except its status and emailed_at.
type Invoice struct {
	ID           int64  `gorm:"column:id;primary_key" json:"id"`
	Number       string `gorm:"column:number" json:"number"`
	Type         string `gorm:"column:type" json:"type"`
	RefInvoiceID *int64 `gorm:"column:ref_invoice_id" json:"refInvoiceId"`
	OrderID      int64  `gorm:"column:order_id" json:"orderId"`
	OrderNumber  string `gorm:"column:order_number" json:"orderNumber"`
	Status       string `gorm:"column:status" json:"status"`
	MerchantID   string `gorm:"column:merchant_id" json:"merchantId"`
	OutletID     string `gorm:"column:outlet_id" json:"outletId"`

	SellerName    string `gorm:"column:seller_name" json:"sellerName"`
	SellerAddress string `gorm:"column:seller_address" json:"sellerAddress"`
	SellerTaxID   string `gorm:"column:seller_tax_id" json:"sellerTaxId"`
	CustomerName  string `gorm:"column:customer_name" json:"customerName"`
	CustomerEmail string `gorm:"column:customer_email" json:"customerEmail"`
	CustomerPhone string `gorm:"column:customer_phone" json:"customerPhone"`

	Currency           string  `gorm:"column:currency" json:"currency"`
	Subtotal           int64   `gorm:"column:subtotal" json:"subtotal"`
	TaxName            string  `gorm:"column:tax_name" json:"taxName"`
	TaxRate            float64 `gorm:"column:tax_rate" json:"taxRate"`
	TaxMode            string  `gorm:"column:tax_mode" json:"taxMode"`
	TaxAmount          int64   `gorm:"column:tax_amount" json:"taxAmount"`
	RoundingAdjustment int64   `gorm:"column:rounding_adjustment" json:"roundingAdjustment"`
	Fee                int64   `gorm:"column:fee" json:"fee"`
	TotalAmount        int64   `gorm:"column:total_amount" json:"totalAmount"`
	MethodCode         string  `gorm:"column:method_code" json:"methodCode"`

	PaidAt    *time.Time `gorm:"column:paid_at" json:"paidAt"`
	IssuedAt  time.Time  `gorm:"column:issued_at" json:"issuedAt"`
	EmailedAt *time.Time `gorm:"column:emailed_at" json:"emailedAt"`
	CreatedAt time.Time  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt time.Time  `gorm:"column:updated_at" json:"updatedAt"`

	Items []InvoiceItem `gorm:"-" json:"items,omitempty"`
}

// TableName return table name of Invoice model
func (Invoice) TableName() string { return "invoices" }

// InvoiceItem is a snapshot of an order line on an invoice
type InvoiceItem struct {
	ID        int64  `gorm:"column:id;primary_key" json:"-"`
	InvoiceID int64  `gorm:"column:invoice_id" json:"-"`
	LineNo    int    `gorm:"column:line_no" json:"lineNo"`
	Name      string `gorm:"column:name" json:"name"`
	Price     int64  `gorm:"column:price" json:"price"`
	Quantity  int    `gorm:"column:quantity" json:"quantity"`
	LineTotal int64  `gorm:"column:line_total" json:"lineTotal"`
}

// TableName return table name of InvoiceItem model
func (InvoiceItem) TableName() string { return "invoice_items" }
