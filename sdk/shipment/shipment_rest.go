package shipment

import (
	"net/http"
	"time"

	"github.com/golangid/candi/candiutils"
)

type shipmentRESTImpl struct {
	host    string
	authKey string
	httpReq candiutils.HTTPRequest
}

// NewShipmentServiceREST constructor
func NewShipmentServiceREST(host string, authKey string) Shipment {

	return &shipmentRESTImpl{
		host:    host,
		authKey: authKey,
		httpReq: candiutils.NewHTTPRequest(
			candiutils.HTTPRequestSetRetries(5),
			candiutils.HTTPRequestSetSleepBetweenRetry(500*time.Millisecond),
			candiutils.HTTPRequestSetHTTPErrorCodeThreshold(http.StatusBadRequest),
			candiutils.HTTPRequestSetBreakerName("shipment"),
		),
	}
}
