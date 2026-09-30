package domain

// PriceLine is a basket line to price
type PriceLine struct {
	Name     string `json:"name"`
	Price    int64  `json:"price"`
	Quantity int    `json:"quantity"`
}

// Breakdown is the priced basket: what the payment amount is expected to be
type Breakdown struct {
	Lines              []OrderItem `json:"lines"`
	Subtotal           int64       `json:"subtotal"`
	TaxName            string      `json:"taxName"`
	TaxRate            float64     `json:"taxRate"`
	TaxMode            string      `json:"taxMode"`
	TaxAmount          int64       `json:"taxAmount"`
	RoundingMode       string      `json:"roundingMode"`
	RoundingUnit       int64       `json:"roundingUnit"`
	RoundingAdjustment int64       `json:"roundingAdjustment"`
	Total              int64       `json:"total"`
	Currency           string      `json:"currency"`
}
