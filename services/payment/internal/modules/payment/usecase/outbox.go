package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/golangid/candi/candishared"
	"github.com/golangid/candi/logger"
	"github.com/golangid/candi/tracer"
)

const outboxBatch = 50

// kickOutbox flushes the outbox right after a commit so events do not wait for the cron.
// The cron flush still covers a crash between commit and this call, and Kafka outages.
func (uc *paymentUsecaseImpl) kickOutbox() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.LogE("payment: outbox kick panic")
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := uc.FlushOutbox(ctx); err != nil {
			logger.LogIf("payment: outbox flush: %v", err)
		}
	}()
}

// FlushOutbox publishes unpublished outbox events to the Kafka topics configured in the DB.
// Rows are locked SKIP LOCKED, so several instances can flush at once without duplicates.
// An event whose type has no enabled topic is marked done and dropped (topic switched off on purpose).
func (uc *paymentUsecaseImpl) FlushOutbox(ctx context.Context) (published int, err error) {
	if !uc.flushMu.TryLock() {
		return 0, nil // this instance is already flushing
	}
	defer uc.flushMu.Unlock()

	trace, ctx := tracer.StartTraceWithContext(ctx, "PaymentUsecase:FlushOutbox")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = uc.repoSQL.WithTransaction(ctx, func(ctx context.Context) error {
		repo := uc.repoSQL.PaymentRepo()
		rows, err := repo.LockPendingOutbox(ctx, outboxBatch)
		if err != nil {
			return err
		}
		for i := range rows {
			row := &rows[i]
			now := uc.now()
			topics, err := uc.repoSQL.TopicRepo().FetchEnabledPublish(ctx, row.EventType)
			if err != nil {
				return err
			}
			if len(topics) == 0 {
				row.PublishedAt, row.LastError = &now, "no enabled topic for event type, dropped"
				if err = repo.SaveOutbox(ctx, row); err != nil {
					return err
				}
				continue
			}
			var perr error
			for _, t := range topics {
				if perr = uc.publish(ctx, t.Topic, row.Key, row.Payload); perr != nil {
					break
				}
			}
			if perr != nil {
				row.Attempts++
				row.LastError = truncateReason(perr.Error())
				if err = repo.SaveOutbox(ctx, row); err != nil {
					return err
				}
				logger.LogIf("payment: publish outbox %d (%s) failed, retrying later: %v", row.ID, row.EventType, perr)
				return nil // stop the batch to keep order, the attempt counter is committed
			}
			row.PublishedAt, row.LastError = &now, ""
			if err = repo.SaveOutbox(ctx, row); err != nil {
				return err
			}
			published++
		}
		return nil
	})
	return published, err
}

func (uc *paymentUsecaseImpl) publish(ctx context.Context, topic, key string, payload []byte) error {
	pub := uc.publisher()
	if pub == nil {
		return errors.New("kafka publisher is not configured")
	}
	return pub.PublishMessage(ctx, &candishared.PublisherArgument{
		Topic: topic, Key: key, Message: payload, ContentType: "application/json",
	})
}
