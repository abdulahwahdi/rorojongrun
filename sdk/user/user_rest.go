package user

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/golangid/candi/candiutils"
)

type userRESTImpl struct {
	host    string
	authKey string
	httpReq candiutils.HTTPRequest
}

// NewUserServiceREST constructor
func NewUserServiceREST(host string, authKey string) User {

	return &userRESTImpl{
		host:    host,
		authKey: authKey,
		httpReq: candiutils.NewHTTPRequest(
			candiutils.HTTPRequestSetRetries(5),
			candiutils.HTTPRequestSetSleepBetweenRetry(500*time.Millisecond),
			candiutils.HTTPRequestSetHTTPErrorCodeThreshold(http.StatusBadRequest),
			candiutils.HTTPRequestSetBreakerName("user"),
		),
	}
}

// errRESTUnsupported: the internal authorization API is gRPC only
var errRESTUnsupported = errors.New("user: the internal authorization API is only available over gRPC, use NewUserServiceGRPC")

func (u *userRESTImpl) CheckPermission(ctx context.Context, req CheckPermissionRequest) (res CheckPermissionResponse, err error) {
	return res, errRESTUnsupported
}

func (u *userRESTImpl) GetUser(ctx context.Context, realm string, id int) (res UserResponse, err error) {
	return res, errRESTUnsupported
}

func (u *userRESTImpl) GetUserPermissions(ctx context.Context, realm string, userID int, service string) ([]string, error) {
	return nil, errRESTUnsupported
}
