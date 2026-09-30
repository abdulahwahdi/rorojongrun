package domain

// Activity is an entry of the activity service's audit trail, the payload of activity.requested
// (services/activity/internal/modules/activity/domain.RequestSaveActivity)
type Activity struct {
	ServiceName string         `json:"serviceName"`
	EventType   string         `json:"eventType"`
	ReferenceID string         `json:"referenceId"`
	ActorID     string         `json:"actorId,omitempty"`
	Message     string         `json:"message,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// OrderActivity is an activity entry about an order, with the ids to find it again
func OrderActivity(eventType string, o *Order, actor, message string, extra map[string]any) Activity {
	meta := map[string]any{
		"orderId": o.ID, "orderNumber": o.OrderNumber, "paymentId": o.PaymentID, "merchantId": o.MerchantID,
		"outletId": o.OutletID, "paymentStatus": o.PaymentStatus, "orderStatus": o.OrderStatus,
	}
	for k, v := range extra {
		meta[k] = v
	}
	return Activity{EventType: eventType, ReferenceID: o.OrderNumber, ActorID: actor, Message: message, Metadata: meta}
}
