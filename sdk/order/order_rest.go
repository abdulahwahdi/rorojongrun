package order

import (
	"net/http"
	"time"

	"github.com/golangid/candi/candiutils"
)

type orderRESTImpl struct {
	host    string
	authKey string
	httpReq candiutils.HTTPRequest
}

// NewOrderServiceREST constructor
func NewOrderServiceREST(host string, authKey string) Order {

	return &orderRESTImpl{
		host:    host,
		authKey: authKey,
		httpReq: candiutils.NewHTTPRequest(
			candiutils.HTTPRequestSetRetries(5),
			candiutils.HTTPRequestSetSleepBetweenRetry(500*time.Millisecond),
			candiutils.HTTPRequestSetHTTPErrorCodeThreshold(http.StatusBadRequest),
			candiutils.HTTPRequestSetBreakerName("order"),
		),
	}
}
