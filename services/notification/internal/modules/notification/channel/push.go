package channel

import (
	"context"
	"os"

	sharedpkg "monorepo/services/notification/pkg/shared"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"

	"github.com/golangid/candi/logger"
)

// PushSender sends push notifications via Firebase Cloud Messaging.
// recipient is an FCM device token, not an email address.
type PushSender struct {
	client *messaging.Client
}

// NewPushSender constructor. If FCM credentials aren't configured/readable,
// it returns a sender that logs a "would send" message instead of hard
// failing — this keeps local dev usable without real Firebase credentials.
func NewPushSender() *PushSender {
	env := sharedpkg.GetEnv()
	if env.FCMCredentialsJSONPath == "" {
		logger.LogI("push channel: FCM_CREDENTIALS_JSON_PATH not set, push notifications will be logged only")
		return &PushSender{}
	}
	if _, err := os.Stat(env.FCMCredentialsJSONPath); err != nil {
		logger.LogEf("push channel: FCM credentials not readable at %s, push notifications will be logged only: %v", env.FCMCredentialsJSONPath, err)
		return &PushSender{}
	}

	app, err := firebase.NewApp(context.Background(), nil, option.WithCredentialsFile(env.FCMCredentialsJSONPath))
	if err != nil {
		logger.LogEf("push channel: failed to init firebase app, push notifications will be logged only: %v", err)
		return &PushSender{}
	}
	client, err := app.Messaging(context.Background())
	if err != nil {
		logger.LogEf("push channel: failed to init messaging client, push notifications will be logged only: %v", err)
		return &PushSender{}
	}

	return &PushSender{client: client}
}

// Send delivers a push notification. recipient is an FCM device token.
func (s *PushSender) Send(ctx context.Context, recipient, subject, body string) error {
	if s.client == nil {
		logger.LogIf("push channel: would send to token=%s title=%q body=%q (no FCM client configured)", recipient, subject, body)
		return nil
	}

	_, err := s.client.Send(ctx, &messaging.Message{
		Token: recipient,
		Notification: &messaging.Notification{
			Title: subject,
			Body:  body,
		},
	})
	return err
}
