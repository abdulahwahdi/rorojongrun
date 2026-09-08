package payment

import (
	"net/http"
	"time"

	"github.com/golangid/candi/candiutils"
)

type paymentRESTImpl struct {
	host    string
	authKey string
	httpReq candiutils.HTTPRequest
}

// NewPaymentServiceREST constructor
func NewPaymentServiceREST(host string, authKey string) Payment {

	return &paymentRESTImpl{
		host:    host,
		authKey: authKey,
		httpReq: candiutils.NewHTTPRequest(
			candiutils.HTTPRequestSetRetries(5),
			candiutils.HTTPRequestSetSleepBetweenRetry(500*time.Millisecond),
			candiutils.HTTPRequestSetHTTPErrorCodeThreshold(http.StatusBadRequest),
			candiutils.HTTPRequestSetBreakerName("payment"),
		),
	}
}
