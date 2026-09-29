package domain

// AllowedCredentialKeys per gateway; unknown keys are rejected so typos do not silently disable auth
var AllowedCredentialKeys = map[string][]string{
	"midtrans": {"serverKey", "clientKey"},
	"xendit":   {"secretKey", "callbackToken"},
	"mock":     {"callbackToken"},
}

// RequiredCredentialKeys must all be present before a gateway can be enabled
var RequiredCredentialKeys = map[string][]string{
	"midtrans": {"serverKey"},
	"xendit":   {"secretKey", "callbackToken"},
	"mock":     {},
}
