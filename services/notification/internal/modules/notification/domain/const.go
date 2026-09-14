package domain

// TaskNameNotificationDispatch is the task queue worker task name used to
// enqueue (from the usecase) and mount (from the worker handler) the async
// notification-dispatch job. Keep these in sync.
const TaskNameNotificationDispatch = "notification-dispatch-task"

// Channel identifiers, shared across templates, channel configs, and logs.
const (
	ChannelEmail = "email"
	ChannelPush  = "push"
)

// StatusPending / StatusSent / StatusFailed are the notification_logs.status values.
const (
	StatusPending = "pending"
	StatusSent    = "sent"
	StatusFailed  = "failed"
)
