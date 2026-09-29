package workerhandler

import (
	"monorepo/services/order/internal/modules/order/domain"
	"monorepo/services/order/pkg/shared/usecase"

	"github.com/golangid/candi/candishared"
	cronworker "github.com/golangid/candi/codebase/app/cron_worker"
	"github.com/golangid/candi/codebase/factory/dependency"
	"github.com/golangid/candi/codebase/factory/types"
	"github.com/golangid/candi/codebase/interfaces"
	"github.com/golangid/candi/logger"
	"github.com/golangid/candi/tracer"
)

// CronHandler struct
type CronHandler struct {
	uc        usecase.Usecase
	validator interfaces.Validator
}

// NewCronHandler constructor
func NewCronHandler(uc usecase.Usecase, deps dependency.Dependency) *CronHandler {
	return &CronHandler{uc: uc, validator: deps.GetValidator()}
}

// MountHandlers mount handler group
func (h *CronHandler) MountHandlers(group *types.WorkerHandlerGroup) {
	group.Add(cronworker.CreateCronJobKey(domain.JobFlushOutbox, "", "10s"), h.handleFlushOutbox)
	group.Add(cronworker.CreateCronJobKey(domain.JobSweepExports, "", "1m"), h.handleSweepExports)
	group.Add(cronworker.CreateCronJobKey(domain.JobPurgeExports, "", "1h"), h.handlePurgeExports)
}

// handleSweepExports queues again exports the task queue lost and takes over silent ones
func (h *CronHandler) handleSweepExports(eventContext *candishared.EventContext) error {
	trace, ctx := tracer.StartTraceWithContext(eventContext.Context(), "OrderDeliveryCron:SweepExports")
	defer trace.Finish()

	n, err := h.uc.Export().SweepExports(ctx)
	if n > 0 {
		logger.LogIf("order: queued %d exports again", n)
	}
	return err
}

// handlePurgeExports deletes the files of expired exports
func (h *CronHandler) handlePurgeExports(eventContext *candishared.EventContext) error {
	trace, ctx := tracer.StartTraceWithContext(eventContext.Context(), "OrderDeliveryCron:PurgeExports")
	defer trace.Finish()

	_, err := h.uc.Export().PurgeExports(ctx)
	return err
}

// handleFlushOutbox is the safety net that publishes outbox events that were not published
// right after their commit (crash, Kafka outage)
func (h *CronHandler) handleFlushOutbox(eventContext *candishared.EventContext) error {
	trace, ctx := tracer.StartTraceWithContext(eventContext.Context(), "OrderDeliveryCron:FlushOutbox")
	defer trace.Finish()

	_, err := h.uc.Order().FlushOutbox(ctx)
	return err
}
