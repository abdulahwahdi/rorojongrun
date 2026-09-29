package shared

import (
	"strings"
	"time"
)

// Environment additional in this service
type Environment struct {
	// CHECKOUT_BASE_URL is where the checkout frontend lives; the payment link is <base>/<token>
	CheckoutBaseURL string `env:"CHECKOUT_BASE_URL"`
	// GATEWAY_ENCRYPTION_SECRET encrypts gateway credentials at rest (AES-256-GCM).
	// Changing it makes stored credentials unreadable: re-enter them afterwards.
	GatewayEncryptionSecret string `env:"GATEWAY_ENCRYPTION_SECRET"`
	// DEFAULT_PAYMENT_EXPIRY is how long a payment link stays payable when the caller gives no expiry
	DefaultPaymentExpiry time.Duration `env:"DEFAULT_PAYMENT_EXPIRY" optional:"true"`

	// USE_CALLBACK_CONSUMER runs the DB-driven gateway callback consumer (topics come from payment_kafka_topics)
	UseCallbackConsumer         bool          `env:"USE_CALLBACK_CONSUMER" optional:"true"`
	CallbackConsumerGroup       string        `env:"CALLBACK_CONSUMER_GROUP" optional:"true"`
	CallbackTopicReloadInterval time.Duration `env:"CALLBACK_TOPIC_RELOAD_INTERVAL" optional:"true"`

	// Enables the mock gateway's dev endpoint that publishes a callback. Refused when ENVIRONMENT=production.
	Environment string `env:"ENVIRONMENT" optional:"true"`

	// Token validation and permission checks go through the user service (globalshared/auth)
	IssuerBaseURL    string `env:"ISSUER_BASE_URL"`
	UserHTTPHost     string `env:"USER_HTTP_HOST"`
	UserGRPCHost     string `env:"USER_GRPC_HOST"`
	UserBasicAuthKey string `env:"USER_BASIC_AUTH_KEY" optional:"true"`
}

var sharedEnv = withDefaults(Environment{})

func withDefaults(env Environment) Environment {
	if env.DefaultPaymentExpiry <= 0 {
		env.DefaultPaymentExpiry = 24 * time.Hour
	}
	if env.CallbackConsumerGroup == "" {
		env.CallbackConsumerGroup = "payment-callback"
	}
	if env.CallbackTopicReloadInterval <= 0 {
		env.CallbackTopicReloadInterval = 30 * time.Second
	}
	env.CheckoutBaseURL = strings.TrimRight(env.CheckoutBaseURL, "/")
	return env
}

// GetEnv get global additional environment
func GetEnv() Environment {
	return sharedEnv
}

// SetEnv get global additional environment
func SetEnv(env Environment) {
	sharedEnv = withDefaults(env)
}

// IsProduction reports ENVIRONMENT=production
func (e Environment) IsProduction() bool { return e.Environment == "production" }
