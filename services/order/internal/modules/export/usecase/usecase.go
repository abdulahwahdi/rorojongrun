package usecase

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"monorepo/services/order/internal/modules/export/domain"
	"monorepo/services/order/pkg/shared"
	shareddomain "monorepo/services/order/pkg/shared/domain"
	"monorepo/services/order/pkg/shared/repository"
	"monorepo/services/order/pkg/shared/usecase/common"

	taskqueueworker "github.com/golangid/candi/codebase/app/task_queue_worker"
	"github.com/golangid/candi/codebase/factory/dependency"
	"github.com/google/uuid"
)

// Caller is who asks: their subject, and whether they may act on everybody's exports (manageExports)
type Caller struct {
	Subject string
	Manager bool
}

// ExportUsecase abstraction: asynchronous CSV exports of orders, order lines and invoices
type ExportUsecase interface {
	RequestExport(ctx context.Context, caller Caller, req *domain.RequestCreateExport) (res shareddomain.ExportJob, err error)
	GetAllExports(ctx context.Context, caller Caller, filter *domain.FilterExport) (res domain.ResponseExportList, err error)
	GetExport(ctx context.Context, caller Caller, id string) (res shareddomain.ExportJob, err error)
	CancelExport(ctx context.Context, caller Caller, id string) (res shareddomain.ExportJob, err error)
	// DownloadExport opens the CSV of a completed export; the caller closes the reader
	DownloadExport(ctx context.Context, caller Caller, id string) (job shareddomain.ExportJob, file io.ReadCloser, size int64, err error)

	// RunExport generates the CSV of a job (task queue worker)
	RunExport(ctx context.Context, id string) (err error)
	// SweepExports queues stuck jobs again, failing those out of attempts (cron)
	SweepExports(ctx context.Context) (requeued int, err error)
	// PurgeExports deletes the files of expired exports (cron)
	PurgeExports(ctx context.Context) (purged int, err error)
}

type exportUsecaseImpl struct {
	deps          dependency.Dependency
	sharedUsecase common.Usecase
	repoSQL       repository.RepoSQL

	// swappable in tests
	now     func() time.Time
	env     func() shared.Environment
	files   func() shared.FileStore
	newID   func() string
	enqueue func(ctx context.Context, jobID string) error
}

// NewExportUsecase usecase impl constructor
func NewExportUsecase(deps dependency.Dependency) (ExportUsecase, func(sharedUsecase common.Usecase)) {
	uc := &exportUsecaseImpl{
		deps: deps, repoSQL: repository.GetSharedRepoSQL(), now: time.Now, env: shared.GetEnv,
		files: func() shared.FileStore { return shared.NewLocalFileStore(shared.GetEnv().ExportStorageDir) },
		newID: uuid.NewString, enqueue: addTask,
	}
	return uc, func(sharedUsecase common.Usecase) {
		uc.sharedUsecase = sharedUsecase
	}
}

// addTask hands the job to candi's task queue (it only accepts jobs where USE_TASK_QUEUE_WORKER runs)
func addTask(ctx context.Context, jobID string) error {
	args, _ := json.Marshal(jobID)
	_, err := taskqueueworker.AddJob(ctx, &taskqueueworker.AddJobRequest{
		TaskName: domain.TaskExport, Args: args, MaxRetry: domain.MaxAttempts, RetryInterval: 10 * time.Second,
	})
	return err
}
