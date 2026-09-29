package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"monorepo/services/payment/internal/modules/payment/domain"
	"monorepo/services/payment/pkg/helper"
	shareddomain "monorepo/services/payment/pkg/shared/domain"

	"github.com/golangid/candi/logger"
)

func (uc *paymentUsecaseImpl) paymentURL(p *shareddomain.Payment) string {
	base := uc.env().CheckoutBaseURL
	if base == "" {
		return ""
	}
	return base + "/" + p.Token
}

func (uc *paymentUsecaseImpl) paymentEvent(event string, p *shareddomain.Payment, txn *shareddomain.Transaction) domain.PaymentEvent {
	ev := domain.PaymentEvent{
		Event: event, PaymentID: p.ID, Source: p.Source, ReferenceID: p.ReferenceID, Status: p.Status,
		Amount: p.Amount, Fee: p.Fee, TotalAmount: p.TotalAmount, Currency: p.Currency, MethodCode: p.MethodCode,
		PaidAt: utcPtr(p.PaidAt), ExpiresAt: p.ExpiresAt.UTC(), Metadata: decodeMap(p.Metadata),
	}
	if txn != nil {
		ev.TransactionID, ev.MethodCode, ev.GatewayCode = txn.ID, txn.MethodCode, txn.GatewayCode
		ev.Fee, ev.TotalAmount = txn.Amount-p.Amount, txn.Amount
	}
	return ev
}

// enqueue writes an event to the outbox. It must run inside the transaction of the state
// change it describes so the event exists if and only if the change committed.
func (uc *paymentUsecaseImpl) enqueue(ctx context.Context, eventType, key string, payload any) error {
	return uc.repoSQL.PaymentRepo().SaveOutbox(ctx, &shareddomain.Outbox{
		EventType: eventType, Key: key, Payload: shareddomain.JSON(jsonBytes(payload)),
	})
}

func (uc *paymentUsecaseImpl) enqueueEvent(ctx context.Context, event string, p *shareddomain.Payment, txn *shareddomain.Transaction) error {
	return uc.enqueue(ctx, event, p.ID, uc.paymentEvent(event, p, txn))
}

// enqueueEmail queues a customer email for the notification service. A payment without
// a customer email simply gets none.
func (uc *paymentUsecaseImpl) enqueueEmail(ctx context.Context, templateCode string, p *shareddomain.Payment, txn *shareddomain.Transaction) error {
	var customer shareddomain.Customer
	_ = p.Customer.Decode(&customer)
	if customer.Email == "" {
		logger.LogIf("payment %s: no customer email, skipping %s email", p.ID, templateCode)
		return nil
	}

	methodName := p.MethodCode
	if m, err := uc.repoSQL.MethodRepo().FindByCode(ctx, p.MethodCode); err == nil {
		methodName = m.Name
	}
	name := customer.Name
	if name == "" {
		name = "Customer"
	}
	total := p.TotalAmount
	if txn != nil {
		total = txn.Amount
	}
	vars := map[string]any{
		"customerName": name, "referenceId": p.ReferenceID, "description": p.Description,
		"totalAmount": helper.FormatIDR(total), "amount": helper.FormatIDR(p.Amount),
		"fee": helper.FormatIDR(total - p.Amount), "methodName": methodName,
		"expiresAt": formatTime(p.ExpiresAt), "paymentUrl": uc.paymentURL(p),
	}
	if txn != nil {
		vars["instructionLines"] = instructionLines(decodeMap(txn.Instruction), uc.paymentURL(p))
	}
	if p.PaidAt != nil {
		vars["paidAt"] = formatTime(*p.PaidAt)
	}
	return uc.enqueue(ctx, shareddomain.EventNotificationMail, p.ID, domain.NotificationRequest{
		Channel: "email", TemplateCode: templateCode, Recipient: customer.Email, Variables: vars,
	})
}

func formatTime(t time.Time) string { return t.In(jakarta).Format("2 Jan 2006 15:04 MST") }

// instructionLines turns a gateway-independent instruction into lines a customer can follow
func instructionLines(ins map[string]any, paymentURL string) []string {
	str := func(k string) string { s, _ := ins[k].(string); return s }
	var lines []string
	if v := str("vaNumber"); v != "" {
		if b := str("bank"); b != "" {
			lines = append(lines, "Bank: "+strings.ToUpper(b))
		}
		lines = append(lines, "Virtual account number: "+v)
	}
	if v := str("billKey"); v != "" {
		lines = append(lines, "Biller code: "+str("billerCode"), "Bill key: "+v)
	}
	if v := str("cashCode"); v != "" {
		lines = append(lines, "Cash payment code: "+v, "Show this code to the cashier and pay the total amount in cash.")
	}
	if str("qrString") != "" || str("qrUrl") != "" {
		lines = append(lines, "Scan the QR code on your payment page"+suffixURL(paymentURL))
	}
	if v := str("url"); v != "" {
		lines = append(lines, "Complete your payment here: "+v)
	}
	if v := str("deeplinkUrl"); v != "" {
		lines = append(lines, "Open your e-wallet app: "+v)
	}
	if len(lines) == 0 {
		lines = append(lines, fmt.Sprintf("Open your payment page to continue%s", suffixURL(paymentURL)))
	}
	return lines
}

func suffixURL(u string) string {
	if u == "" {
		return ""
	}
	return ": " + u
}

// utcPtr keeps event timestamps in UTC whatever zone the database session uses
func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}
