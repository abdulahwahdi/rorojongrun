package channel

import (
	"context"
	"errors"
)

// Sender delivers a rendered notification over one transport (email, push).
// Sender itself has no concept of "disabled" — that's a config-layer check
// the caller (notification usecase) makes before ever reaching a Sender.
type Sender interface {
	// Send delivers subject+body to recipient. For push, recipient is an
	// FCM device token, not an email address — subject is used as the push
	// notification's title.
	Send(ctx context.Context, recipient, subject, body string) error
}

// ErrChannelDisabled is returned by the notification usecase (not by a
// Sender) when notification_channel_configs.is_enabled is false for the
// requested channel.
var ErrChannelDisabled = errors.New("channel is disabled")
