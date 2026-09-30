package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"monorepo/globalshared/crypto"
	"monorepo/globalshared/rest"
	"monorepo/services/payment/internal/modules/gateway/provider"
	"monorepo/services/payment/internal/modules/payment/domain"
	shareddomain "monorepo/services/payment/pkg/shared/domain"

	"github.com/golangid/candi/candishared"
	"github.com/golangid/candi/logger"
	"github.com/golangid/candi/tracer"
	"gorm.io/gorm"
)

// ConsumeTopics returns the topics to subscribe to: enabled consume topics of enabled gateways
func (uc *paymentUsecaseImpl) ConsumeTopics(ctx context.Context) (topics []string, err error) {
	rows, err := uc.repoSQL.TopicRepo().FetchEnabledConsume(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		topics = append(topics, r.Topic)
	}
	return topics, nil
}

// HandleCallback verifies and applies one gateway callback. Permanent problems (unknown topic,
// bad signature, amount mismatch, unknown transaction) are logged and reported through the
// outcome with a nil error, so the message is not retried. A returned error is transient (DB
// down, ...): the consumer retries and finally calls RecordCallbackFailure.
func (uc *paymentUsecaseImpl) HandleCallback(ctx context.Context, msg *domain.CallbackMessage) (out domain.CallbackOutcome, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PaymentUsecase:HandleCallback")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	topic, err := uc.repoSQL.TopicRepo().FindConsumeByTopic(ctx, msg.Topic)
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && topic.GatewayCode == nil) {
		return uc.logCallback(ctx, msg, "", domain.CallbackOutcome{Status: shareddomain.CallbackFailed, Message: "topic is not a configured consume topic"}), nil
	}
	if err != nil {
		return out, err
	}
	gwCode := *topic.GatewayCode

	row, err := uc.repoSQL.GatewayRepo().FindByCode(ctx, gwCode)
	if err != nil {
		return out, err
	}
	prov, cfg, err := uc.providers(row, false)
	if err != nil {
		return uc.logCallback(ctx, msg, gwCode, domain.CallbackOutcome{Status: shareddomain.CallbackFailed, Message: err.Error()}), nil
	}
	parsed, err := prov.ParseCallback(ctx, cfg, msg.Headers, msg.Body)
	if err != nil {
		return uc.logCallback(ctx, msg, gwCode, domain.CallbackOutcome{Status: shareddomain.CallbackFailed, Message: err.Error()}), nil
	}

	out, err = uc.applyCallback(ctx, gwCode, parsed)
	if err != nil {
		return out, err
	}
	return uc.logCallback(ctx, msg, gwCode, out), nil
}

// RecordCallbackFailure logs a callback that failed after all retries so it can be replayed
func (uc *paymentUsecaseImpl) RecordCallbackFailure(ctx context.Context, msg *domain.CallbackMessage, cause error) {
	gw := ""
	if t, err := uc.repoSQL.TopicRepo().FindConsumeByTopic(ctx, msg.Topic); err == nil && t.GatewayCode != nil {
		gw = *t.GatewayCode
	}
	uc.logCallback(ctx, msg, gw, domain.CallbackOutcome{Status: shareddomain.CallbackFailed, Message: "processing failed: " + cause.Error()})
}

func (uc *paymentUsecaseImpl) logCallback(ctx context.Context, msg *domain.CallbackMessage, gateway string, out domain.CallbackOutcome) domain.CallbackOutcome {
	entry := shareddomain.CallbackLog{
		Topic: msg.Topic, Partition: msg.Partition, Offset: msg.Offset, GatewayCode: gateway,
		ExternalID: out.TransactionID, Headers: uc.sealHeaders(msg.Headers), Payload: string(msg.Body),
		Status: out.Status, Error: truncateReason(out.Message),
	}
	if isValidUUID(out.TransactionID) {
		entry.TransactionID = &out.TransactionID
	}
	if err := uc.repoSQL.PaymentRepo().SaveCallbackLog(ctx, &entry); err != nil {
		logger.LogE("payment: cannot save callback log: " + err.Error())
	}
	return out
}

// sealHeaders stores the headers that authenticate a callback encrypted, so a replay can be
// verified again without keeping those secrets readable in the database
func (uc *paymentUsecaseImpl) sealHeaders(h map[string]string) shareddomain.JSON {
	out := make(map[string]string, len(h))
	for k, v := range h {
		if sensitiveHeaders[k] {
			if enc, err := crypto.Encrypt(uc.env().GatewayEncryptionSecret, []byte(v)); err == nil {
				v = sealedPrefix + enc
			} else {
				v = "[redacted]"
			}
		}
		out[k] = v
	}
	return shareddomain.NewJSON(out)
}

func (uc *paymentUsecaseImpl) openHeaders(j shareddomain.JSON) map[string]string {
	h := map[string]string{}
	_ = j.Decode(&h)
	for k, v := range h {
		if isSealed(v) {
			if plain, err := crypto.Decrypt(uc.env().GatewayEncryptionSecret, v[len(sealedPrefix):]); err == nil {
				h[k] = string(plain)
			}
		}
	}
	return h
}

// redactHeaders is the API view of stored headers
func redactHeaders(j shareddomain.JSON) shareddomain.JSON {
	h := map[string]string{}
	_ = j.Decode(&h)
	for k, v := range h {
		if isSealed(v) {
			h[k] = "[redacted]"
		}
	}
	return shareddomain.NewJSON(h)
}

// applyCallback moves the payment according to a verified callback. Idempotent: the same
// callback delivered twice, or a late one for a settled payment, changes nothing.
func (uc *paymentUsecaseImpl) applyCallback(ctx context.Context, gatewayCode string, cb provider.CallbackResult) (out domain.CallbackOutcome, err error) {
	out = domain.CallbackOutcome{TransactionID: cb.TransactionID}
	if cb.Status == provider.CallbackIgnored {
		out.Status, out.Message = shareddomain.CallbackIgnored, "notification does not change payment state"
		return out, nil
	}
	if !isValidUUID(cb.TransactionID) {
		out.Status, out.Message = shareddomain.CallbackIgnored, "unknown transaction"
		return out, nil
	}
	repo := uc.repoSQL.PaymentRepo()
	found, err := repo.FindTransactionByID(ctx, cb.TransactionID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		out.Status, out.Message = shareddomain.CallbackIgnored, "unknown transaction"
		return out, nil
	}
	if err != nil {
		return out, err
	}

	var dropped []shareddomain.Transaction
	err = uc.repoSQL.WithTransaction(ctx, func(ctx context.Context) error {
		p, err := repo.LockPayment(ctx, found.PaymentID)
		if err != nil {
			return err
		}
		txn, err := repo.FindTransactionByID(ctx, found.ID) // re-read under the lock
		if err != nil {
			return err
		}
		if txn.GatewayCode != gatewayCode {
			out.Status, out.Message = shareddomain.CallbackFailed, fmt.Sprintf("transaction belongs to gateway %q, not %q", txn.GatewayCode, gatewayCode)
			return nil
		}

		switch cb.Status {
		case provider.CallbackPaid:
			out, dropped, err = uc.applyPaid(ctx, &p, &txn, cb, out)
		default: // failed | expired
			out, err = uc.applyNotPaid(ctx, &p, &txn, cb, out)
		}
		return err
	})
	if err != nil {
		return out, err
	}
	uc.cancelAtGateway(ctx, dropped...)
	uc.afterCommit()
	return out, nil
}

func (uc *paymentUsecaseImpl) applyPaid(ctx context.Context, p *shareddomain.Payment, txn *shareddomain.Transaction, cb provider.CallbackResult, out domain.CallbackOutcome) (domain.CallbackOutcome, []shareddomain.Transaction, error) {
	if cb.Amount != txn.Amount {
		out.Status = shareddomain.CallbackFailed
		out.Message = fmt.Sprintf("amount mismatch: gateway reports %d, transaction expects %d", cb.Amount, txn.Amount)
		return out, nil, nil
	}
	if txn.Status == shareddomain.TransactionPaid {
		out.Status, out.Message = shareddomain.CallbackDuplicate, "transaction already paid"
		return out, nil, nil
	}
	paidAt := cb.PaidAt
	if paidAt.IsZero() {
		paidAt = uc.now()
	}

	switch p.Status {
	case shareddomain.PaymentPaid, shareddomain.PaymentCancelled:
		// the customer's money moved but the payment cannot complete: record the attempt as
		// paid for reconciliation and flag it, a refund is needed
		txn.Status, txn.PaidAt = shareddomain.TransactionPaid, &paidAt
		if err := uc.repoSQL.PaymentRepo().SaveTransaction(ctx, txn); err != nil {
			return out, nil, err
		}
		out.Status = shareddomain.CallbackIgnored
		out.Message = "payment is already " + p.Status + ": money received for this transaction, refund required"
		logger.LogIf("payment: REFUND REQUIRED payment=%s transaction=%s (%s)", p.ID, txn.ID, p.Status)
		return out, nil, nil
	}

	dropped, err := uc.markPaid(ctx, p, txn, paidAt)
	if err != nil {
		return out, nil, err
	}
	out.Status = shareddomain.CallbackProcessed
	return out, dropped, nil
}

func (uc *paymentUsecaseImpl) applyNotPaid(ctx context.Context, p *shareddomain.Payment, txn *shareddomain.Transaction, cb provider.CallbackResult, out domain.CallbackOutcome) (domain.CallbackOutcome, error) {
	if txn.Status != shareddomain.TransactionPending {
		out.Status, out.Message = shareddomain.CallbackIgnored, "transaction is already "+txn.Status
		return out, nil
	}
	status := shareddomain.TransactionFailed
	if cb.Status == provider.CallbackExpired {
		status = shareddomain.TransactionExpired
	}
	repo := uc.repoSQL.PaymentRepo()
	if err := uc.closeTransaction(ctx, txn, status, truncateReason(cb.Reason)); err != nil {
		return out, err
	}
	// the customer may try another method, unless this was the last thing keeping the payment open
	if p.Status == shareddomain.PaymentProcessing {
		if _, other := uc.openTransaction(ctx, p.ID); !other {
			p.Status = shareddomain.PaymentPending
			if err := repo.SavePayment(ctx, p); err != nil {
				return out, err
			}
		}
	}
	out.Status = shareddomain.CallbackProcessed
	return out, nil
}

// ReplayCallback processes a logged callback again, e.g. after fixing a gateway config or
// after a failure that was not the message's fault. Signature checks run again.
func (uc *paymentUsecaseImpl) ReplayCallback(ctx context.Context, logID int) (out domain.CallbackOutcome, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PaymentUsecase:ReplayCallback")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	entry, err := uc.repoSQL.PaymentRepo().FindCallbackLogByID(ctx, logID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return out, rest.NewNotFound("callback log not found")
	}
	if err != nil {
		return out, err
	}
	if entry.Status == shareddomain.CallbackProcessed {
		return out, rest.NewConflict("this callback was already processed")
	}
	return uc.HandleCallback(ctx, &domain.CallbackMessage{
		Topic: entry.Topic, Partition: entry.Partition, Offset: entry.Offset,
		Headers: uc.openHeaders(entry.Headers), Body: []byte(entry.Payload),
	})
}

func (uc *paymentUsecaseImpl) GetAllCallbackLogs(ctx context.Context, filter *domain.FilterCallbackLog) (res domain.ResponseCallbackLogList, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PaymentUsecase:GetAllCallbackLogs")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	repo := uc.repoSQL.PaymentRepo()
	if res.Data, err = repo.FetchAllCallbackLogs(ctx, filter); err != nil {
		return res, err
	}
	if res.Data == nil {
		res.Data = []shareddomain.CallbackLog{}
	}
	for i := range res.Data {
		res.Data[i].Headers = redactHeaders(res.Data[i].Headers)
	}
	res.Meta = candishared.NewMeta(filter.Page, filter.Limit, repo.CountCallbackLogs(ctx, filter))
	return res, nil
}

func (uc *paymentUsecaseImpl) GetCallbackLog(ctx context.Context, id int) (res shareddomain.CallbackLog, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PaymentUsecase:GetCallbackLog")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	res, err = uc.repoSQL.PaymentRepo().FindCallbackLogByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return res, rest.NewNotFound("callback log not found")
	}
	res.Headers = redactHeaders(res.Headers)
	return res, err
}

// SimulateMockCallback publishes a callback of the mock gateway to its Kafka topic. Together
// with the mock gateway it lets the whole callback path run without a real gateway. Dev only.
func (uc *paymentUsecaseImpl) SimulateMockCallback(ctx context.Context, req *domain.RequestMockCallback) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PaymentUsecase:SimulateMockCallback")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	if uc.env().IsProduction() {
		return rest.NewForbidden("the mock gateway is disabled in production")
	}
	if !isValidUUID(req.TransactionID) {
		return rest.NewNotFound("transaction not found")
	}
	txn, err := uc.repoSQL.PaymentRepo().FindTransactionByID(ctx, req.TransactionID)
	if err != nil {
		return rest.NewNotFound("transaction not found")
	}
	if txn.GatewayCode != shareddomain.GatewayMock {
		return rest.NewInvalid("transaction does not belong to the mock gateway")
	}

	row, err := uc.repoSQL.GatewayRepo().FindByCode(ctx, shareddomain.GatewayMock)
	if err != nil {
		return err
	}
	cfg, err := provider.BuildConfig(row, uc.env().GatewayEncryptionSecret)
	if err != nil {
		return err
	}
	topics, err := uc.repoSQL.TopicRepo().FetchEnabledConsume(ctx)
	if err != nil {
		return err
	}
	topic := ""
	for _, t := range topics {
		if t.GatewayCode != nil && *t.GatewayCode == shareddomain.GatewayMock {
			topic = t.Topic
			break
		}
	}
	if topic == "" {
		return rest.NewUnavailable("no enabled consume topic for the mock gateway (is the gateway enabled?)")
	}

	headers := map[string]string{}
	if token := cfg.Credentials["callbackToken"]; token != "" {
		headers["x-mock-token"] = token
	}
	body := jsonBytes(map[string]any{"transactionId": txn.ID, "status": req.Status, "amount": txn.Amount})
	envelope := jsonBytes(map[string]any{"headers": headers, "body": json.RawMessage(body), "sentAt": time.Now().Format(time.RFC3339)})
	return uc.publish(ctx, topic, txn.ID, envelope)
}
