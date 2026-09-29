package workerhandler

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"monorepo/services/order/internal/modules/order/domain"
	ucorder "monorepo/services/order/internal/modules/order/usecase"
	"monorepo/services/order/pkg/shared"
	"monorepo/services/order/pkg/shared/usecase"

	"github.com/golangid/candi/candishared"
	"github.com/golangid/candi/codebase/factory/dependency"
	"github.com/golangid/candi/codebase/factory/types"
	"github.com/golangid/candi/codebase/interfaces"
	"github.com/golangid/candi/logger"
	"github.com/golangid/candi/tracer"
)

// KafkaHandler struct
type KafkaHandler struct {
	uc        usecase.Usecase
	validator interfaces.Validator
	// retry policy for transient errors (candi marks the message consumed even when a handler fails)
	attempts int
	backoff  time.Duration
}

// NewKafkaHandler constructor
func NewKafkaHandler(uc usecase.Usecase, deps dependency.Dependency) *KafkaHandler {
	return &KafkaHandler{uc: uc, validator: deps.GetValidator(), attempts: 3, backoff: time.Second}
}

// MountHandlers mount handler group. Every PAYMENT_EVENT_TOPICS topic goes to one handler that
// dispatches on the payload's "event" field, so the topic names only live in the env.
func (h *KafkaHandler) MountHandlers(group *types.WorkerHandlerGroup) {
	for _, topic := range shared.GetEnv().Topics() {
		group.Add(topic, h.handlePaymentEvent)
	}
}

func (h *KafkaHandler) handlePaymentEvent(eventContext *candishared.EventContext) error {
	trace, ctx := tracer.StartTraceWithContext(eventContext.Context(), "OrderDeliveryKafka:HandlePaymentEvent")
	defer trace.Finish()

	var ev domain.PaymentEvent
	if err := json.Unmarshal(eventContext.Message(), &ev); err != nil {
		logger.LogE("order: skipping undecodable payment event on " + eventContext.HandlerRoute() + ": " + err.Error())
		return nil // a poison message must not block the partition
	}
	src := domain.KafkaSource{Topic: eventContext.HandlerRoute()}
	if p, err := strconv.Atoi(eventContext.Header()["partition"]); err == nil {
		src.Partition = int32(p)
	}
	src.Offset, _ = strconv.ParseInt(eventContext.Header()["offset"], 10, 64)

	var err error
	for attempt := 1; attempt <= h.attempts; attempt++ {
		if err = h.uc.Order().RecordPaymentEvent(ctx, &ev, src); err == nil {
			return nil
		}
		if errors.Is(err, ucorder.ErrBadEvent) {
			logger.LogE("order: skipping " + err.Error())
			return nil
		}
		if attempt < h.attempts {
			time.Sleep(h.backoff * time.Duration(attempt))
		}
	}
	// the message is committed anyway; replaying it (reset offsets) is safe, booking is idempotent
	logger.LogE("order: failed to book " + ev.Event + " of payment " + ev.PaymentID + " at " +
		src.Topic + "/" + strconv.Itoa(int(src.Partition)) + "/" + strconv.FormatInt(src.Offset, 10) + ": " + err.Error())
	trace.SetError(err)
	return err
}
