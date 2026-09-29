package notification

import (
	"context"
	"errors"
	"net/url"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
)

// errGRPCNotAvailable is returned by every method — the notification
// service has no gRPC handler enabled (candi.json: GRPCHandler=false), so
// there is no proto/server to call. Fill these in for real if that changes
// via `candi -add-handler -service=notification`.
var errGRPCNotAvailable = errors.New("notification: grpc client not available, service has no grpc handler enabled")

type notificationGRPCImpl struct {
	host    string
	authKey string
	conn    *grpc.ClientConn
}

// NewNotificationServiceGRPC constructor
func NewNotificationServiceGRPC(host string, authKey string) Notification {

	if u, _ := url.Parse(host); u.Host != "" {
		host = u.Host
	}
	conn, err := grpc.NewClient(host, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithConnectParams(grpc.ConnectParams{
		Backoff: backoff.Config{
			BaseDelay:  50 * time.Millisecond,
			Multiplier: 5,
			MaxDelay:   50 * time.Millisecond,
		},
		MinConnectTimeout: 1 * time.Second,
	}))
	if err != nil {
		panic(err)
	}

	return &notificationGRPCImpl{
		host:    host,
		authKey: authKey,
		conn:    conn,
	}
}

func (r *notificationGRPCImpl) SendNotification(ctx context.Context, req SendNotificationRequest) (jobID string, err error) {
	return "", errGRPCNotAvailable
}

func (r *notificationGRPCImpl) GetAllNotificationLogs(ctx context.Context, filter GetNotificationLogsFilter) (NotificationLogListResponse, error) {
	return NotificationLogListResponse{}, errGRPCNotAvailable
}
