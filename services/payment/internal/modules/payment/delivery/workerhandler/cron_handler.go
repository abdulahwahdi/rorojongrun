package workerhandler

import (
	"monorepo/services/payment/internal/modules/payment/domain"
	"monorepo/services/payment/pkg/shared/usecase"

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
	group.Add(cronworker.CreateCronJobKey(domain.JobExpirePayments, "", "1m"), h.handleExpirePayments)
	group.Add(cronworker.CreateCronJobKey(domain.JobFlushOutbox, "", "10s"), h.handleFlushOutbox)
}

// handleExpirePayments expires overdue payments and their open attempts
func (h *CronHandler) handleExpirePayments(eventContext *candishared.EventContext) error {
	trace, ctx := tracer.StartTraceWithContext(eventContext.Context(), "PaymentDeliveryCron:ExpirePayments")
	defer trace.Finish()

	n, err := h.uc.Payment().ExpireOverduePayments(ctx)
	if n > 0 {
		logger.LogIf("payment: expired %d overdue payments", n)
	}
	return err
}

// handleFlushOutbox is the safety net that publishes outbox events that were not
// published right after their commit (crash, Kafka outage)
func (h *CronHandler) handleFlushOutbox(eventContext *candishared.EventContext) error {
	trace, ctx := tracer.StartTraceWithContext(eventContext.Context(), "PaymentDeliveryCron:FlushOutbox")
	defer trace.Finish()

	_, err := h.uc.Payment().FlushOutbox(ctx)
	return err
}
