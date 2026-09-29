package domain

// Supported grant types of the token endpoint
const (
	GrantPassword          = "password"
	GrantRefreshToken      = "refresh_token"
	GrantClientCredentials = "client_credentials"
	GrantOTP               = "otp"
)

// OTPPurposePrefix + realm name is the purpose the OTP is requested / verified under in the notification service
const OTPPurposePrefix = "login:"
