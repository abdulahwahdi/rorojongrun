package domain

// ResponseRequestOTP model — returned immediately by the request endpoint
type ResponseRequestOTP struct {
	JobID string `json:"jobId"`
}

// ResponseVerifyOTP model
type ResponseVerifyOTP struct {
	Verified bool `json:"verified"`
}
