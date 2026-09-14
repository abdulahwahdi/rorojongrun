package domain

// RequestOTP is the payload for requesting a new OTP code.
type RequestOTP struct {
	Recipient string `json:"recipient"`
	Channel   string `json:"channel"`
	Purpose   string `json:"purpose"`
}

// RequestVerifyOTP is the payload for verifying a submitted OTP code.
type RequestVerifyOTP struct {
	Recipient string `json:"recipient"`
	Purpose   string `json:"purpose"`
	Code      string `json:"code"`
}
