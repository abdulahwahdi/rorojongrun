package repository

import (
	"context"
	"time"

	"monorepo/services/order/internal/modules/export/domain"
	shareddomain "monorepo/services/order/pkg/shared/domain"
)

// ExportRepository abstract interface
type ExportRepository interface {
	Create(ctx context.Context, data *shareddomain.ExportJob) error
	Update(ctx context.Context, data *shareddomain.ExportJob) error
	FindByID(ctx context.Context, id string) (shareddomain.ExportJob, error)
	// LockByID reads a job with SELECT ... FOR UPDATE; call inside WithTransaction
	LockByID(ctx context.Context, id string) (shareddomain.ExportJob, error)
	// Claim moves a queued job to running and counts the attempt; false when it is not queued
	Claim(ctx context.Context, id string, now time.Time) (bool, error)
	// Progress records the rows written and the heartbeat, and reads back whether a cancel was asked
	Progress(ctx context.Context, id string, processed int64, now time.Time) (cancelRequested bool, err error)
	FetchAll(ctx context.Context, filter *domain.FilterExport) ([]shareddomain.ExportJob, error)
	Count(ctx context.Context, filter *domain.FilterExport) int
	// FetchStale lists queued jobs untouched since queuedBefore and running jobs silent since heartbeatBefore
	FetchStale(ctx context.Context, queuedBefore, heartbeatBefore time.Time, limit int) ([]shareddomain.ExportJob, error)
	// FetchExpired lists completed jobs whose download window ended
	FetchExpired(ctx context.Context, now time.Time, limit int) ([]shareddomain.ExportJob, error)
}
