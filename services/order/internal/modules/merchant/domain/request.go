package domain

// RequestSaveMerchant creates or replaces the settings of a merchant
type RequestSaveMerchant struct {
	Name             string  `json:"name"`
	Address          string  `json:"address"`
	TaxID            string  `json:"taxId"`
	OrderPrefix      string  `json:"orderPrefix"`
	InvoicePrefix    string  `json:"invoicePrefix"`
	CreditNotePrefix string  `json:"creditNotePrefix"`
	TaxName          string  `json:"taxName"`
	TaxRate          float64 `json:"taxRate"`
	TaxMode          string  `json:"taxMode"`
	RoundingMode     string  `json:"roundingMode"`
	RoundingUnit     int64   `json:"roundingUnit"`
	Timezone         string  `json:"timezone"`
}
