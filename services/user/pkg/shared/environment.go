package shared

// Environment additional in this service
type Environment struct {
	// ISSUER_BASE_URL is the public base url of this service; tokens carry "iss" = <base>/realms/<realm>
	IssuerBaseURL string `env:"ISSUER_BASE_URL"`
	// KEY_ENCRYPTION_SECRET encrypts realm private signing keys at rest (AES-256-GCM)
	KeyEncryptionSecret string `env:"KEY_ENCRYPTION_SECRET"`

	// Admin created in the "master" realm when it has no users yet
	BootstrapAdminUsername string `env:"BOOTSTRAP_ADMIN_USERNAME" optional:"true"`
	BootstrapAdminPassword string `env:"BOOTSTRAP_ADMIN_PASSWORD" optional:"true"`

	// NotificationHost is the http base url of the notification service (OTP login)
	NotificationHost    string `env:"NOTIFICATION_HOST" optional:"true"`
	NotificationAuthKey string `env:"NOTIFICATION_AUTH_KEY" optional:"true"`
}

var sharedEnv Environment

// GetEnv get global additional environment
func GetEnv() Environment {
	return sharedEnv
}

// SetEnv get global additional environment
func SetEnv(env Environment) {
	sharedEnv = env
}
