package user

import (
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
