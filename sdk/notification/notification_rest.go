package notification

import (
	"net/http"
	"time"

	"github.com/golangid/candi/candiutils"
)

type notificationRESTImpl struct {
	host    string
	authKey string
	httpReq candiutils.HTTPRequest
}

// NewNotificationServiceREST constructor
func NewNotificationServiceREST(host string, authKey string) Notification {

	return &notificationRESTImpl{
		host:    host,
		authKey: authKey,
		httpReq: candiutils.NewHTTPRequest(
			candiutils.HTTPRequestSetRetries(5),
			candiutils.HTTPRequestSetSleepBetweenRetry(500*time.Millisecond),
			candiutils.HTTPRequestSetHTTPErrorCodeThreshold(http.StatusBadRequest),
			candiutils.HTTPRequestSetBreakerName("notification"),
		),
	}
}
