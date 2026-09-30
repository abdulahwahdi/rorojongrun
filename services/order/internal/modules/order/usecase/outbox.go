package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	shareddomain "monorepo/services/order/pkg/shared/domain"

	"github.com/golangid/candi/candishared"
	"github.com/golangid/candi/logger"
	"github.com/golangid/candi/tracer"
)

const outboxBatch = 50

// Enqueue writes an event to the outbox. It must run inside the transaction of the change it
// describes, so the event exists if and only if the change committed. The topic is the event
// name (`<service>.<event>`), except customer emails which go to ORDER_NOTIFICATION_TOPIC.
func (uc *orderUsecaseImpl) Enqueue(ctx context.Context, eventType, key string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	topic := eventType
	switch eventType {
	case shareddomain.EventNotificationEmail:
		topic = uc.env().NotificationTopic
	case shareddomain.EventActivityLog:
		topic = uc.env().ActivityTopic
	}
	return uc.repoSQL.OutboxRepo().Save(ctx, &shareddomain.Outbox{
		EventType: eventType, Topic: topic, Key: key, Payload: shareddomain.JSON(b),
	})
}

// LogActivity queues an audit trail entry for the activity service
func (uc *orderUsecaseImpl) LogActivity(ctx context.Context, entry shareddomain.Activity) error {
	entry.ServiceName = "order"
	return uc.Enqueue(ctx, shareddomain.EventActivityLog, entry.ReferenceID, entry)
}

// KickOutbox flushes the outbox right after a commit so events do not wait for the cron.
// The cron flush still covers a crash between commit and this call, and Kafka outages.
func (uc *orderUsecaseImpl) KickOutbox() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.LogE("order: outbox kick panic")
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := uc.FlushOutbox(ctx); err != nil {
			logger.LogIf("order: outbox flush: %v", err)
		}
	}()
}

// FlushOutbox publishes unpublished outbox events in order. Rows are locked SKIP LOCKED, so several
// instances can flush at once without duplicates; a failed publish stops the batch to keep order.
func (uc *orderUsecaseImpl) FlushOutbox(ctx context.Context) (published int, err error) {
	if !uc.flushMu.TryLock() {
		return 0, nil // this instance is already flushing
	}
	defer uc.flushMu.Unlock()

	trace, ctx := tracer.StartTraceWithContext(ctx, "OrderUsecase:FlushOutbox")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = uc.repoSQL.WithTransaction(ctx, func(ctx context.Context) error {
		repo := uc.repoSQL.OutboxRepo()
		rows, err := repo.LockPending(ctx, outboxBatch)
		if err != nil {
			return err
		}
		for i := range rows {
			row := &rows[i]
			if perr := uc.publish(ctx, row.Topic, row.Key, row.Payload); perr != nil {
				row.Attempts++
				row.LastError = truncate(perr.Error(), 500)
				if err = repo.Save(ctx, row); err != nil {
					return err
				}
				logger.LogIf("order: publish outbox %d (%s) failed, retrying later: %v", row.ID, row.EventType, perr)
				return nil // the attempt counter is committed
			}
			now := uc.now()
			row.PublishedAt, row.LastError = &now, ""
			if err = repo.Save(ctx, row); err != nil {
				return err
			}
			published++
		}
		return nil
	})
	return published, err
}

func (uc *orderUsecaseImpl) publish(ctx context.Context, topic, key string, payload []byte) error {
	pub := uc.publisher()
	if pub == nil {
		return errors.New("kafka publisher is not configured")
	}
	return pub.PublishMessage(ctx, &candishared.PublisherArgument{
		Topic: topic, Key: key, Message: payload, ContentType: "application/json",
	})
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
