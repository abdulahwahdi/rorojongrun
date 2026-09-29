package workerhandler

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"monorepo/services/payment/internal/modules/payment/domain"
	"monorepo/services/payment/pkg/shared"
	"monorepo/services/payment/pkg/shared/usecase"

	"github.com/IBM/sarama"
	"github.com/golangid/candi/broker"
	"github.com/golangid/candi/codebase/factory"
	"github.com/golangid/candi/codebase/factory/types"
	"github.com/golangid/candi/logger"
	"github.com/golangid/candi/tracer"
)

const (
	callbackAttempts = 3
	consumeBackoff   = 2 * time.Second
)

// CallbackConsumer consumes gateway callbacks from the Kafka topics configured in the
// payment_kafka_topics table. candi's own Kafka worker fixes its topic list when it is built,
// so it cannot follow DB changes; this consumer re-reads the table every
// CALLBACK_TOPIC_RELOAD_INTERVAL (and immediately when an admin API call on this instance
// changes it) and rejoins its consumer group with the new topic set when it differs.
type CallbackConsumer struct {
	uc     func() usecase.Usecase
	group  sarama.ConsumerGroup
	bk     *broker.KafkaBroker
	cancel context.CancelFunc
	done   chan struct{}
}

// NewCallbackConsumer builds the consumer on the service's Kafka broker
func NewCallbackConsumer(service factory.ServiceFactory) factory.AppServerFactory {
	bk, ok := service.GetDependency().GetBroker(types.Kafka).(*broker.KafkaBroker)
	if !ok || bk == nil {
		panic("callback consumer needs the Kafka broker")
	}
	group, err := sarama.NewConsumerGroupFromClient(shared.GetEnv().CallbackConsumerGroup, bk.Client)
	if err != nil {
		panic(fmt.Errorf("callback consumer group: %w", err))
	}
	return &CallbackConsumer{
		uc:    usecase.GetSharedUsecase,
		group: group, bk: bk, done: make(chan struct{}),
	}
}

// Name implements factory.AppServerFactory
func (c *CallbackConsumer) Name() string { return "payment-callback-consumer" }

// Serve implements factory.AppServerFactory
func (c *CallbackConsumer) Serve() {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	defer close(c.done)

	fmt.Printf("\x1b[34;1m⇨ Payment callback consumer running (group %q, topics from DB, reload every %s)\x1b[0m\n\n",
		shared.GetEnv().CallbackConsumerGroup, shared.GetEnv().CallbackTopicReloadInterval)

	for ctx.Err() == nil {
		topics, err := c.uc().Payment().ConsumeTopics(ctx)
		if err != nil {
			logger.LogE("payment callback consumer: load topics: " + err.Error())
			c.wait(ctx, shared.GetEnv().CallbackTopicReloadInterval)
			continue
		}
		if len(topics) == 0 {
			logger.LogYellow("payment callback consumer: no enabled consume topics, waiting")
			c.wait(ctx, shared.GetEnv().CallbackTopicReloadInterval)
			continue
		}
		sort.Strings(topics)
		logger.LogYellow("payment callback consumer: subscribing to " + strings.Join(topics, ", "))

		c.consumeGeneration(ctx, topics)
		if ctx.Err() == nil {
			c.wait(ctx, consumeBackoff)
		}
	}
}

// consumeGeneration consumes one topic set until it goes stale, a rebalance ends the session,
// or the consumer shuts down
func (c *CallbackConsumer) consumeGeneration(ctx context.Context, topics []string) {
	genCtx, cancelGen := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { // reload watcher
		defer wg.Done()
		ticker := time.NewTicker(shared.GetEnv().CallbackTopicReloadInterval)
		defer ticker.Stop()
		for {
			select {
			case <-genCtx.Done():
				return
			case <-ticker.C:
			case <-shared.TopicsChanged():
			}
			latest, err := c.uc().Payment().ConsumeTopics(genCtx)
			if err != nil {
				logger.LogE("payment callback consumer: reload topics: " + err.Error())
				continue
			}
			if !sameTopics(topics, latest) {
				logger.LogYellow("payment callback consumer: topic set changed, resubscribing")
				cancelGen()
				return
			}
		}
	}()

	handler := &callbackHandler{uc: c.uc, brokers: strings.Join(c.bk.BrokerHost, ",")}
	if err := c.group.Consume(genCtx, topics, handler); err != nil && ctx.Err() == nil {
		logger.LogE("payment callback consumer: " + err.Error())
	}
	cancelGen()
	wg.Wait()
}

func (c *CallbackConsumer) wait(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	case <-shared.TopicsChanged():
	}
}

// Shutdown implements factory.AppServerFactory
func (c *CallbackConsumer) Shutdown(ctx context.Context) {
	fmt.Printf("\r%s \x1b[33;1mStopping payment callback consumer:\x1b[0m ... ", time.Now().Format("2006/01/02 15:04:05"))
	if c.cancel != nil {
		c.cancel()
	}
	select {
	case <-c.done:
	case <-ctx.Done():
	}
	_ = c.group.Close()
	fmt.Println("\x1b[32;1mSUCCESS\x1b[0m")
}

type callbackHandler struct {
	uc      func() usecase.Usecase
	brokers string
}

func (h *callbackHandler) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (h *callbackHandler) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (h *callbackHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			h.process(session, msg)
		case <-session.Context().Done():
			return nil
		}
	}
}

// process handles one message and always marks it: a message that keeps failing is recorded
// in payment_callback_logs (replayable through the admin API) instead of blocking the partition
func (h *callbackHandler) process(session sarama.ConsumerGroupSession, msg *sarama.ConsumerMessage) {
	kafkaHeaders := map[string]string{}
	for _, hd := range msg.Headers {
		kafkaHeaders[string(hd.Key)] = string(hd.Value)
	}

	var err error
	mark := true
	trace, ctx := tracer.StartTraceFromHeader(session.Context(), "PaymentCallbackConsumer", kafkaHeaders)
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
		if mark {
			session.MarkMessage(msg, "")
		}
		trace.Finish(tracer.FinishWithError(err))
	}()
	trace.SetTag("brokers", h.brokers)
	trace.SetTag("topic", msg.Topic)
	trace.SetTag("key", msg.Key)

	headers, body := parseEnvelope(msg.Value, kafkaHeaders)
	cb := &domain.CallbackMessage{Topic: msg.Topic, Partition: msg.Partition, Offset: msg.Offset, Headers: headers, Body: body}

	for attempt := 1; attempt <= callbackAttempts; attempt++ {
		var out domain.CallbackOutcome
		if out, err = h.uc().Payment().HandleCallback(ctx, cb); err == nil {
			trace.SetTag("outcome", out.Status)
			if out.Status == "failed" {
				logger.LogIf("payment callback %s[%d]@%d rejected: %s", msg.Topic, msg.Partition, msg.Offset, out.Message)
			}
			return
		}
		logger.LogIf("payment callback %s[%d]@%d attempt %d/%d: %v", msg.Topic, msg.Partition, msg.Offset, attempt, callbackAttempts, err)
		select {
		case <-ctx.Done():
			mark = false // shutting down mid-retry: leave the message uncommitted so it is redelivered
			return
		case <-time.After(time.Duration(attempt) * 200 * time.Millisecond):
		}
	}
	h.uc().Payment().RecordCallbackFailure(ctx, cb, err)
}
