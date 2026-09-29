package workerhandler

import (
	"encoding/json"

	"monorepo/services/order/internal/modules/export/domain"
	"monorepo/services/order/pkg/shared/usecase"

	"github.com/golangid/candi/candishared"
	taskqueueworker "github.com/golangid/candi/codebase/app/task_queue_worker"
	"github.com/golangid/candi/codebase/factory/dependency"
	"github.com/golangid/candi/codebase/factory/types"
	"github.com/golangid/candi/codebase/interfaces"
	"github.com/golangid/candi/logger"
	"github.com/golangid/candi/tracer"
)

// TaskQueueHandler struct
type TaskQueueHandler struct {
	uc        usecase.Usecase
	validator interfaces.Validator
}

// NewTaskQueueHandler constructor
func NewTaskQueueHandler(uc usecase.Usecase, deps dependency.Dependency) *TaskQueueHandler {
	return &TaskQueueHandler{uc: uc, validator: deps.GetValidator()}
}

// MountHandlers mount handler group
func (h *TaskQueueHandler) MountHandlers(group *types.WorkerHandlerGroup) {
	group.Add(domain.TaskExport, h.handleExport,
		types.WorkerHandlerOptionAddConfig(taskqueueworker.TaskOptionDeleteJobAfterSuccess, true),
	)
}

// handleExport generates one export; the export_jobs row is the source of truth, the task only carries its id
func (h *TaskQueueHandler) handleExport(eventContext *candishared.EventContext) error {
	trace, ctx := tracer.StartTraceWithContext(eventContext.Context(), "ExportDeliveryTaskQueue:HandleExport")
	defer trace.Finish()

	var jobID string
	if err := json.Unmarshal(eventContext.Message(), &jobID); err != nil || jobID == "" {
		logger.LogE("order: export task without a job id: " + string(eventContext.Message()))
		return nil
	}
	return h.uc.Export().RunExport(ctx, jobID)
}
