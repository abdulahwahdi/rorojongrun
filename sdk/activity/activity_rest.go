package activity

import (
	"net/http"
	"time"

	"github.com/golangid/candi/candiutils"
)

type activityRESTImpl struct {
	host    string
	authKey string
	httpReq candiutils.HTTPRequest
}

// NewActivityServiceREST constructor
func NewActivityServiceREST(host string, authKey string) Activity {

	return &activityRESTImpl{
		host:    host,
		authKey: authKey,
		httpReq: candiutils.NewHTTPRequest(
			candiutils.HTTPRequestSetRetries(5),
			candiutils.HTTPRequestSetSleepBetweenRetry(500*time.Millisecond),
			candiutils.HTTPRequestSetHTTPErrorCodeThreshold(http.StatusBadRequest),
			candiutils.HTTPRequestSetBreakerName("activity"),
		),
	}
}
