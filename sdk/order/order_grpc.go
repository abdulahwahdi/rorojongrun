package order

import (
	"net/url"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
)

type orderGRPCImpl struct {
	host    string
	authKey string
	conn    *grpc.ClientConn
}

// NewOrderServiceGRPC constructor
func NewOrderServiceGRPC(host string, authKey string) Order {

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

	return &orderGRPCImpl{
		host:    host,
		authKey: authKey,
		conn:    conn,
	}
}
