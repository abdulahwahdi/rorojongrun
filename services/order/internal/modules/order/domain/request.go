package domain

import shareddomain "monorepo/services/order/pkg/shared/domain"

// RequestUpdateStatus moves an order to another order status (staff)
type RequestUpdateStatus struct {
	Status string `json:"status"`
	Note   string `json:"note"`
	// Version is the order version the caller saw; a changed order answers 409
	Version int `json:"version"`
}

// RequestOverridePaymentStatus sets the payment status by hand (admin), e.g. after a lost callback
type RequestOverridePaymentStatus struct {
	Status  string `json:"status"`
	Note    string `json:"note"`
	Version int    `json:"version"`
	// MethodCode records how it was paid when it is set to paid by hand (e.g. cash taken offline)
	MethodCode string `json:"methodCode"`
}

// RequestQuote prices a basket with a merchant's tax and rounding settings
type RequestQuote struct {
	MerchantID string                   `json:"merchantId"`
	Items      []shareddomain.PriceLine `json:"items"`
}
