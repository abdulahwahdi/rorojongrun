package activity

import (
	"context"
	"errors"
	"net/url"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
)

// errGRPCNotAvailable is returned by every method — the activity service has
// no gRPC handler enabled (candi.json: GRPCHandler=false), so there is no
// proto/server to call. Fill these in for real if that changes via
// `candi -add-handler -service=activity`.
var errGRPCNotAvailable = errors.New("activity: grpc client not available, service has no grpc handler enabled")

type activityGRPCImpl struct {
	host    string
	authKey string
	conn    *grpc.ClientConn
}

// NewActivityServiceGRPC constructor
func NewActivityServiceGRPC(host string, authKey string) Activity {

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

	return &activityGRPCImpl{
		host:    host,
		authKey: authKey,
		conn:    conn,
	}
}

func (r *activityGRPCImpl) SaveActivityLog(ctx context.Context, req SaveActivityLogRequest) (jobID string, err error) {
	return "", errGRPCNotAvailable
}

func (r *activityGRPCImpl) GetAllActivityLogs(ctx context.Context, filter GetActivityLogsFilter) (ActivityLogListResponse, error) {
	return ActivityLogListResponse{}, errGRPCNotAvailable
}

func (r *activityGRPCImpl) GetActivityLogByID(ctx context.Context, id string) (ActivityLogResponse, error) {
	return ActivityLogResponse{}, errGRPCNotAvailable
}
