package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"monorepo/globalshared/rest"
	"monorepo/services/order/internal/modules/export/domain"
	shareddomain "monorepo/services/order/pkg/shared/domain"

	"github.com/golangid/candi/candishared"
	"github.com/golangid/candi/logger"
	"github.com/golangid/candi/tracer"
	"gorm.io/gorm"
)

// ExportPermission is the permission code needed to request an export of a type
func ExportPermission(exportType string) string {
	if exportType == shareddomain.ExportInvoices {
		return "exportInvoices"
	}
	return "exportOrders"
}

// decodeFilter reads the stored filter of a job and resolves its dates; merchant is the filter's merchant
func (uc *exportUsecaseImpl) decodeFilter(ctx context.Context, exportType string, raw []byte) (orders *shareddomain.FilterOrder, invoices *shareddomain.FilterInvoice, merchant, tz string, err error) {
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	switch exportType {
	case shareddomain.ExportOrders, shareddomain.ExportOrderLines:
		orders = &shareddomain.FilterOrder{}
		if err = json.Unmarshal(raw, orders); err != nil {
			return nil, nil, "", "", rest.NewInvalid("filter: " + err.Error())
		}
		merchant = orders.MerchantID
		tz, err = uc.sharedUsecase.ResolveDates(ctx, merchant, &orders.DateRange)
	case shareddomain.ExportInvoices:
		invoices = &shareddomain.FilterInvoice{}
		if err = json.Unmarshal(raw, invoices); err != nil {
			return nil, nil, "", "", rest.NewInvalid("filter: " + err.Error())
		}
		merchant = invoices.MerchantID
		tz, err = uc.sharedUsecase.ResolveDates(ctx, merchant, &invoices.DateRange)
	default:
		err = rest.NewInvalid("type is orders, order_lines or invoices")
	}
	return
}

func (uc *exportUsecaseImpl) RequestExport(ctx context.Context, caller Caller, req *domain.RequestCreateExport) (job shareddomain.ExportJob, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "ExportUsecase:RequestExport")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	orders, invoices, merchant, _, err := uc.decodeFilter(ctx, req.Type, req.Filter)
	if err != nil {
		return job, err
	}
	var total int
	if orders != nil {
		total = uc.repoSQL.OrderRepo().Count(ctx, orders)
	} else {
		total = uc.repoSQL.InvoiceRepo().Count(ctx, invoices)
	}
	if max := uc.env().ExportMaxRows; int64(total) > max {
		return job, rest.NewInvalid(fmt.Sprintf("the filter matches %d rows, more than the %d an export can hold; narrow it (e.g. a date range)", total, max))
	}

	filter := req.Filter
	if len(filter) == 0 {
		filter = json.RawMessage("{}")
	}
	job = shareddomain.ExportJob{
		ID: uc.newID(), Type: req.Type, Filter: shareddomain.JSON(filter), Status: shareddomain.ExportQueued,
		RequestedBy: caller.Subject, MerchantID: merchant, TotalRows: int64(total),
	}
	err = uc.repoSQL.WithTransaction(ctx, func(ctx context.Context) error {
		if err := uc.repoSQL.ExportRepo().Create(ctx, &job); err != nil {
			return err
		}
		return uc.sharedUsecase.LogActivity(ctx, exportActivity(shareddomain.EventExportRequested, &job, caller.Subject,
			fmt.Sprintf("Export of %d %s requested", total, req.Type)))
	})
	if err != nil {
		return job, err
	}
	uc.sharedUsecase.KickOutbox()
	if qerr := uc.enqueue(ctx, job.ID); qerr != nil { // the sweeper queues it again
		logger.LogE(fmt.Sprintf("order: export %s not queued yet (the sweeper retries): %v", job.ID, qerr))
	}
	return job, nil
}

// find loads a job the caller may see: their own, or any with manageExports
func (uc *exportUsecaseImpl) find(ctx context.Context, caller Caller, id string) (job shareddomain.ExportJob, err error) {
	job, err = uc.repoSQL.ExportRepo().FindByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && !caller.Manager && job.RequestedBy != caller.Subject) {
		return job, rest.NewNotFound("export not found")
	}
	return job, err
}

func (uc *exportUsecaseImpl) GetAllExports(ctx context.Context, caller Caller, filter *domain.FilterExport) (res domain.ResponseExportList, err error) {
	if !caller.Manager {
		filter.RequestedBy = caller.Subject
	}
	if res.Data, err = uc.repoSQL.ExportRepo().FetchAll(ctx, filter); err != nil {
		return
	}
	if res.Data == nil {
		res.Data = []shareddomain.ExportJob{}
	}
	res.Meta = candishared.NewMeta(filter.Page, filter.Limit, uc.repoSQL.ExportRepo().Count(ctx, filter))
	return
}

func (uc *exportUsecaseImpl) GetExport(ctx context.Context, caller Caller, id string) (shareddomain.ExportJob, error) {
	return uc.find(ctx, caller, id)
}

func (uc *exportUsecaseImpl) CancelExport(ctx context.Context, caller Caller, id string) (job shareddomain.ExportJob, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "ExportUsecase:CancelExport")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	if _, err = uc.find(ctx, caller, id); err != nil {
		return job, err
	}
	err = uc.repoSQL.WithTransaction(ctx, func(ctx context.Context) error {
		if job, err = uc.repoSQL.ExportRepo().LockByID(ctx, id); err != nil {
			return err
		}
		switch job.Status {
		case shareddomain.ExportCancelled:
			return nil // twice is fine
		case shareddomain.ExportQueued:
			now := uc.now().UTC()
			job.Status, job.FinishedAt, job.CancelRequested = shareddomain.ExportCancelled, &now, true
		case shareddomain.ExportRunning:
			if job.CancelRequested {
				return nil
			}
			job.CancelRequested = true // the worker stops at its next batch
		default:
			return rest.NewConflict("the export already " + job.Status)
		}
		if err := uc.repoSQL.ExportRepo().Update(ctx, &job); err != nil {
			return err
		}
		return uc.sharedUsecase.LogActivity(ctx, exportActivity(shareddomain.EventExportCancelled, &job, caller.Subject, "Export cancelled"))
	})
	if err == nil {
		uc.sharedUsecase.KickOutbox()
	}
	return job, err
}

func (uc *exportUsecaseImpl) DownloadExport(ctx context.Context, caller Caller, id string) (job shareddomain.ExportJob, file io.ReadCloser, size int64, err error) {
	if job, err = uc.find(ctx, caller, id); err != nil {
		return
	}
	switch job.Status {
	case shareddomain.ExportCompleted:
	case shareddomain.ExportExpired:
		return job, nil, 0, &rest.AppError{Status: http.StatusGone, Message: "the export expired, request it again"}
	default:
		return job, nil, 0, rest.NewConflict("the export is " + job.Status + ", not ready to download")
	}
	if job.ExpiresAt != nil && uc.now().After(*job.ExpiresAt) {
		return job, nil, 0, &rest.AppError{Status: http.StatusGone, Message: "the export expired, request it again"}
	}
	if file, size, err = uc.files().Open(job.FileName); err != nil {
		return
	}
	// best effort audit, a download does not change state
	if lerr := uc.repoSQL.WithTransaction(ctx, func(ctx context.Context) error {
		return uc.sharedUsecase.LogActivity(ctx, exportActivity(shareddomain.EventExportDownloaded, &job, caller.Subject, "Export downloaded"))
	}); lerr == nil {
		uc.sharedUsecase.KickOutbox()
	}
	return
}

func exportActivity(event string, job *shareddomain.ExportJob, actor, message string) shareddomain.Activity {
	return shareddomain.Activity{EventType: event, ReferenceID: "export-" + job.ID, ActorID: actor, Message: message,
		Metadata: map[string]any{"exportId": job.ID, "type": job.Type, "status": job.Status, "totalRows": job.TotalRows,
			"merchantId": job.MerchantID, "filter": job.Filter}}
}
