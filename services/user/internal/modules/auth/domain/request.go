package domain

// RequestToken is the payload of the token endpoint. Which fields are used depends on GrantType:
//   - password:           clientId, username (or email), password
//   - refresh_token:      clientId, refreshToken
//   - client_credentials: clientId, clientSecret
//   - otp:                clientId, email, code
//
// Confidential clients must always send clientSecret.
type RequestToken struct {
	GrantType    string `json:"grantType"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	RefreshToken string `json:"refreshToken"`
	Email        string `json:"email"`
	Code         string `json:"code"`
}

// RequestOTPLogin asks for a login OTP to be sent to an email address
type RequestOTPLogin struct {
	ClientID string `json:"clientId"`
	Email    string `json:"email"`
}

// ClientMeta is what the delivery layer knows about the caller of the token endpoint
type ClientMeta struct {
	IP        string
	UserAgent string
}

// CheckPermissionRequest asks whether a principal may use a permission code of a service
type CheckPermissionRequest struct {
	Realm     string
	UserID    string
	SessionID int
	Service   string
	Code      string
}
