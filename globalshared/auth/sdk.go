package auth

import (
	"context"

	"monorepo/sdk/user"
)

type sdkPermissionClient struct{ client user.User }

// NewSDKPermissionClient adapts the user service sdk client to PermissionClient
func NewSDKPermissionClient(client user.User) PermissionClient {
	return &sdkPermissionClient{client: client}
}

func (c *sdkPermissionClient) CheckPermission(ctx context.Context, req PermissionRequest) (bool, string, error) {
	res, err := c.client.CheckPermission(ctx, user.CheckPermissionRequest{
		Realm: req.Realm, UserID: req.UserID, SessionID: req.SessionID, Service: req.Service, Code: req.Code,
	})
	return res.Allowed, res.Role, err
}
