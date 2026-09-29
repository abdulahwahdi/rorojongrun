package domain

// RequestOpenShift opens a cashier's shift at an outlet
type RequestOpenShift struct {
	MerchantID string `json:"merchantId"`
	OutletID   string `json:"outletId"`
	// CashierID defaults to the caller (token subject); an operator may open a shift for a cashier
	CashierID    string `json:"cashierId"`
	OpeningFloat int64  `json:"openingFloat"`
	Note         string `json:"note"`
}

// RequestCloseShift closes a shift with the cash counted in the drawer
type RequestCloseShift struct {
	CountedCash int64  `json:"countedCash"`
	Note        string `json:"note"`
}
