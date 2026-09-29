package user

import (
	"context"
	"net/url"
	"time"

	proto "monorepo/sdk/user/proto/auth"

	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type userGRPCImpl struct {
	host    string
	authKey string
	conn    *grpc.ClientConn
	client  proto.AuthHandlerClient
}

// NewUserServiceGRPC constructor
func NewUserServiceGRPC(host string, authKey string) User {

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

	return &userGRPCImpl{
		host:    host,
		authKey: authKey,
		conn:    conn,
		client:  proto.NewAuthHandlerClient(conn),
	}
}

// authCtx attaches the internal basic auth key expected by the user service's gRPC server
func (u *userGRPCImpl) authCtx(ctx context.Context) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Basic "+u.authKey)
}

func (u *userGRPCImpl) CheckPermission(ctx context.Context, req CheckPermissionRequest) (res CheckPermissionResponse, err error) {
	resp, err := u.client.CheckPermission(u.authCtx(ctx), &proto.CheckPermissionRequest{
		Realm: req.Realm, UserId: req.UserID, SessionId: int64(req.SessionID), Service: req.Service, Code: req.Code,
	})
	if err != nil {
		return res, err
	}
	return CheckPermissionResponse{Allowed: resp.Allowed, Role: resp.Role}, nil
}

func (u *userGRPCImpl) GetUser(ctx context.Context, realm string, id int) (res UserResponse, err error) {
	resp, err := u.client.GetUser(u.authCtx(ctx), &proto.GetUserRequest{Realm: realm, Id: int64(id)})
	if err != nil {
		return res, err
	}
	return UserResponse{
		ID: int(resp.Id), Realm: resp.Realm, Username: resp.Username, Email: resp.Email, Phone: resp.Phone,
		FullName: resp.FullName, Status: resp.Status, Roles: resp.Roles,
	}, nil
}

func (u *userGRPCImpl) GetUserPermissions(ctx context.Context, realm string, userID int, service string) ([]string, error) {
	resp, err := u.client.GetUserPermissions(u.authCtx(ctx), &proto.GetUserPermissionsRequest{
		Realm: realm, UserId: int64(userID), Service: service,
	})
	if err != nil {
		return nil, err
	}
	return resp.Codes, nil
}
