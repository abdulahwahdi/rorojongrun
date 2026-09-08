package kitchen

import (
	"net/http"
	"time"

	"github.com/golangid/candi/candiutils"
)

type kitchenRESTImpl struct {
	host    string
	authKey string
	httpReq candiutils.HTTPRequest
}

// NewKitchenServiceREST constructor
func NewKitchenServiceREST(host string, authKey string) Kitchen {

	return &kitchenRESTImpl{
		host:    host,
		authKey: authKey,
		httpReq: candiutils.NewHTTPRequest(
			candiutils.HTTPRequestSetRetries(5),
			candiutils.HTTPRequestSetSleepBetweenRetry(500*time.Millisecond),
			candiutils.HTTPRequestSetHTTPErrorCodeThreshold(http.StatusBadRequest),
			candiutils.HTTPRequestSetBreakerName("kitchen"),
		),
	}
}
