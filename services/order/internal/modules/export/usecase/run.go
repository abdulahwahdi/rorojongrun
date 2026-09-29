package usecase

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"time"

	"monorepo/services/order/internal/modules/export/domain"
	shareddomain "monorepo/services/order/pkg/shared/domain"

	"github.com/golangid/candi/candishared"
	"github.com/golangid/candi/logger"
	"github.com/golangid/candi/tracer"
)

// errCancelled stops a run whose job was cancelled
var errCancelled = errors.New("export cancelled")

// RunExport generates the CSV of a job. A job that is not queued (cancelled, taken by another run)
// is skipped. A failure puts the job back to queued for a retry until it is out of attempts.
func (uc *exportUsecaseImpl) RunExport(ctx context.Context, id string) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "ExportUsecase:RunExport")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	repo := uc.repoSQL.ExportRepo()
	claimed, err := repo.Claim(ctx, id, uc.now().UTC())
	if err != nil || !claimed {
		return err
	}
	job, err := repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	job.FileName = fmt.Sprintf("%s-%s.csv", job.Type, job.ID)

	written, runErr := uc.writeCSV(ctx, &job)
	now := uc.now().UTC()
	job.FinishedAt, job.ProcessedRows = &now, written
	switch {
	case errors.Is(runErr, errCancelled):
		_ = uc.files().Delete(job.FileName)
		job.Status, job.FileName, job.FileSize = shareddomain.ExportCancelled, "", 0
	case runErr != nil:
		_ = uc.files().Delete(job.FileName)
		job.Error, job.FileName, job.FileSize = truncate(runErr.Error(), 500), "", 0
		job.Status, job.FinishedAt = shareddomain.ExportQueued, nil
		if job.Attempts >= domain.MaxAttempts {
			job.Status, job.FinishedAt = shareddomain.ExportFailed, &now
		}
	default:
		expires := now.Add(uc.env().ExportRetention)
		job.Status, job.ExpiresAt, job.Error = shareddomain.ExportCompleted, &expires, ""
	}
	if err = repo.Update(ctx, &job); err != nil {
		return err
	}
	if job.Status == shareddomain.ExportQueued {
		// retried by the task queue; the sweeper also picks it up if the queue lost it
		return &candishared.ErrorRetrier{Delay: 10 * time.Second, Message: "export failed, retrying: " + job.Error}
	}
	if job.Status == shareddomain.ExportFailed {
		logger.LogE(fmt.Sprintf("order: export %s failed after %d attempts: %s", job.ID, job.Attempts, job.Error))
	}
	return nil
}

// writeCSV streams the rows of a job into its file in keyset batches, recording progress and
// stopping when a cancel is asked
func (uc *exportUsecaseImpl) writeCSV(ctx context.Context, job *shareddomain.ExportJob) (written int64, err error) {
	orders, invoices, merchant, _, err := uc.decodeFilter(ctx, job.Type, job.Filter)
	if err != nil {
		return 0, err
	}
	m, err := uc.sharedUsecase.MerchantSettings(ctx, merchant)
	if err != nil {
		return 0, err
	}
	loc := m.Location()

	f, err := uc.files().Create(job.FileName)
	if err != nil {
		return 0, err
	}
	defer func() {
		if cerr := f.Close(); err == nil && cerr != nil {
			err = cerr
		}
	}()
	counter := &countingWriter{w: f}
	w := csv.NewWriter(counter)

	var header []string
	switch job.Type {
	case shareddomain.ExportOrders:
		header = orderHeader
	case shareddomain.ExportOrderLines:
		header = orderLineHeader
	default:
		header = invoiceHeader
	}
	if err = w.Write(header); err != nil {
		return 0, err
	}

	var afterID int64
	for {
		var rows [][]string
		var n int
		if orders != nil {
			batch, err := uc.repoSQL.OrderRepo().FetchAfter(ctx, orders, afterID, domain.BatchSize)
			if err != nil {
				return written, err
			}
			n = len(batch)
			if n == 0 {
				break
			}
			afterID = batch[n-1].ID
			if job.Type == shareddomain.ExportOrderLines {
				if rows, err = uc.orderLineRows(ctx, batch, loc); err != nil {
					return written, err
				}
			} else {
				for i := range batch {
					rows = append(rows, orderRow(&batch[i], loc))
				}
			}
		} else {
			batch, err := uc.repoSQL.InvoiceRepo().FetchAfter(ctx, invoices, afterID, domain.BatchSize)
			if err != nil {
				return written, err
			}
			n = len(batch)
			if n == 0 {
				break
			}
			afterID = batch[n-1].ID
			for i := range batch {
				rows = append(rows, invoiceRow(&batch[i], loc))
			}
		}
		if err = w.WriteAll(rows); err != nil {
			return written, err
		}
		written += int64(len(rows))
		cancelled, err := uc.repoSQL.ExportRepo().Progress(ctx, job.ID, written, uc.now().UTC())
		if err != nil {
			return written, err
		}
		if cancelled {
			return written, errCancelled
		}
		if n < domain.BatchSize {
			break
		}
	}
	w.Flush()
	if err = w.Error(); err != nil {
		return written, err
	}
	job.FileSize = counter.n
	return written, nil
}

func (uc *exportUsecaseImpl) orderLineRows(ctx context.Context, batch []shareddomain.Order, loc *time.Location) ([][]string, error) {
	ids := make([]int64, len(batch))
	byID := make(map[int64]*shareddomain.Order, len(batch))
	for i := range batch {
		ids[i], byID[batch[i].ID] = batch[i].ID, &batch[i]
	}
	items, err := uc.repoSQL.OrderRepo().FetchItems(ctx, ids...)
	if err != nil {
		return nil, err
	}
	rows := make([][]string, 0, len(items))
	for i := range items {
		rows = append(rows, orderLineRow(byID[items[i].OrderID], &items[i], loc))
	}
	return rows, nil
}

// SweepExports queues again the jobs the queue lost (restart, worker not running when requested)
// and takes over running jobs that stopped reporting; a job out of attempts is failed.
func (uc *exportUsecaseImpl) SweepExports(ctx context.Context) (requeued int, err error) {
	now := uc.now().UTC()
	stale, err := uc.repoSQL.ExportRepo().FetchStale(ctx, now.Add(-domain.QueuedGrace), now.Add(-domain.HeartbeatTimeout), 100)
	if err != nil {
		return 0, err
	}
	for i := range stale {
		job := &stale[i]
		if job.CancelRequested {
			job.Status, job.FinishedAt = shareddomain.ExportCancelled, &now
		} else if job.Attempts >= domain.MaxAttempts {
			job.Status, job.FinishedAt = shareddomain.ExportFailed, &now
			if job.Error == "" {
				job.Error = "stopped responding"
			}
		} else {
			job.Status = shareddomain.ExportQueued
		}
		if err = uc.repoSQL.ExportRepo().Update(ctx, job); err != nil {
			return requeued, err
		}
		if job.Status == shareddomain.ExportQueued {
			if qerr := uc.enqueue(ctx, job.ID); qerr != nil {
				logger.LogE(fmt.Sprintf("order: export %s not queued: %v", job.ID, qerr))
				continue
			}
			requeued++
		}
	}
	return requeued, nil
}

// PurgeExports deletes the files of exports whose download window ended
func (uc *exportUsecaseImpl) PurgeExports(ctx context.Context) (purged int, err error) {
	expired, err := uc.repoSQL.ExportRepo().FetchExpired(ctx, uc.now().UTC(), 100)
	if err != nil {
		return 0, err
	}
	for i := range expired {
		job := &expired[i]
		if err = uc.files().Delete(job.FileName); err != nil {
			return purged, err
		}
		job.Status = shareddomain.ExportExpired
		if err = uc.repoSQL.ExportRepo().Update(ctx, job); err != nil {
			return purged, err
		}
		purged++
	}
	return purged, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
