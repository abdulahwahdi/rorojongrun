package shared

import (
	"strings"
	"time"
)

// Environment additional in this service
type Environment struct {
	// PAYMENT_EVENT_TOPICS are the topics carrying the payment service's payment.* events (comma separated).
	// The handler dispatches on the payload's "event" field, so renamed topics only need this list.
	PaymentEventTopics string `env:"PAYMENT_EVENT_TOPICS" optional:"true"`
	// ORDER_NOTIFICATION_TOPIC is where customer emails are asked for (the notification service consumes it)
	NotificationTopic string `env:"ORDER_NOTIFICATION_TOPIC" optional:"true"`
	// ORDER_ACTIVITY_TOPIC is where the audit trail of every order change goes (the activity service consumes it)
	ActivityTopic string `env:"ORDER_ACTIVITY_TOPIC" optional:"true"`

	// EXPORT_STORAGE_DIR holds generated CSV exports (a volume shared by the replicas in k8s)
	ExportStorageDir string `env:"EXPORT_STORAGE_DIR" optional:"true"`
	// EXPORT_RETENTION is how long a completed export can be downloaded
	ExportRetention time.Duration `env:"EXPORT_RETENTION" optional:"true"`
	// ORDER_EXPORT_MAX_ROWS refuses an export request that matches more rows
	ExportMaxRows int64 `env:"ORDER_EXPORT_MAX_ROWS" optional:"true"`

	// Token validation and permission checks go through the user service (globalshared/auth)
	IssuerBaseURL    string `env:"ISSUER_BASE_URL"`
	UserHTTPHost     string `env:"USER_HTTP_HOST"`
	UserGRPCHost     string `env:"USER_GRPC_HOST"`
	UserBasicAuthKey string `env:"USER_BASIC_AUTH_KEY" optional:"true"`
}

// DefaultPaymentEventTopics are the payment service's default publish topics
const DefaultPaymentEventTopics = "payment.created,payment.checkout_started,payment.completed,payment.expired,payment.cancelled"

var sharedEnv = withDefaults(Environment{})

func withDefaults(env Environment) Environment {
	if strings.TrimSpace(env.PaymentEventTopics) == "" {
		env.PaymentEventTopics = DefaultPaymentEventTopics
	}
	if env.NotificationTopic == "" {
		env.NotificationTopic = "notification.requested"
	}
	if env.ActivityTopic == "" {
		env.ActivityTopic = "activity.requested"
	}
	if env.ExportStorageDir == "" {
		env.ExportStorageDir = "./storage/exports"
	}
	if env.ExportRetention <= 0 {
		env.ExportRetention = 7 * 24 * time.Hour
	}
	if env.ExportMaxRows <= 0 {
		env.ExportMaxRows = 100000
	}
	return env
}

// Topics lists PAYMENT_EVENT_TOPICS
func (e Environment) Topics() (topics []string) {
	for _, t := range strings.Split(e.PaymentEventTopics, ",") {
		if t = strings.TrimSpace(t); t != "" {
			topics = append(topics, t)
		}
	}
	return topics
}

// GetEnv get global additional environment
func GetEnv() Environment {
	return sharedEnv
}

// SetEnv get global additional environment
func SetEnv(env Environment) {
	sharedEnv = withDefaults(env)
}
