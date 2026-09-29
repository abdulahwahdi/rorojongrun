package usecase

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"monorepo/services/payment/internal/modules/gateway/provider"
	gatewayrepo "monorepo/services/payment/internal/modules/gateway/repository"
	methoddomain "monorepo/services/payment/internal/modules/method/domain"
	methodrepo "monorepo/services/payment/internal/modules/method/repository"
	"monorepo/services/payment/internal/modules/payment/domain"
	paymentrepo "monorepo/services/payment/internal/modules/payment/repository"
	topicdomain "monorepo/services/payment/internal/modules/topic/domain"
	topicrepo "monorepo/services/payment/internal/modules/topic/repository"
	"monorepo/services/payment/pkg/shared"
	shareddomain "monorepo/services/payment/pkg/shared/domain"
	"monorepo/services/payment/pkg/shared/repository"

	"github.com/golangid/candi/candishared"
	"github.com/golangid/candi/codebase/interfaces"
	"gorm.io/gorm"
)

// ---- in-memory repositories. They implement the real interfaces, so the tests exercise the
// usecase's actual state machine; WithTransaction rolls back on error like the database does.

type memPaymentRepo struct {
	payments map[string]shareddomain.Payment
	txns     map[string]shareddomain.Transaction
	logs     []shareddomain.CallbackLog
	outbox   []shareddomain.Outbox
	nextLog  int
	nextOut  int64
	seq      int
	created  map[string]int // insertion order, keeps "latest" queries deterministic
}

func newMemPaymentRepo() *memPaymentRepo {
	return &memPaymentRepo{payments: map[string]shareddomain.Payment{}, txns: map[string]shareddomain.Transaction{}, created: map[string]int{}}
}

func (r *memPaymentRepo) clone() *memPaymentRepo {
	c := *r
	c.payments = map[string]shareddomain.Payment{}
	for k, v := range r.payments {
		c.payments[k] = v
	}
	c.txns = map[string]shareddomain.Transaction{}
	for k, v := range r.txns {
		c.txns[k] = v
	}
	c.created = map[string]int{}
	for k, v := range r.created {
		c.created[k] = v
	}
	c.logs = append([]shareddomain.CallbackLog(nil), r.logs...)
	c.outbox = append([]shareddomain.Outbox(nil), r.outbox...)
	return &c
}

func (r *memPaymentRepo) SavePayment(_ context.Context, d *shareddomain.Payment) error {
	if d.CreatedAt.IsZero() {
		d.CreatedAt = time.Now()
		for _, p := range r.payments { // mirror the partial unique index on (source, reference_id)
			if p.Source == d.Source && p.ReferenceID == d.ReferenceID && p.ID != d.ID &&
				(p.Status == shareddomain.PaymentPending || p.Status == shareddomain.PaymentProcessing) &&
				(d.Status == shareddomain.PaymentPending || d.Status == shareddomain.PaymentProcessing) {
				return &duplicateErr{}
			}
		}
	}
	r.payments[d.ID] = *d
	return nil
}

type duplicateErr struct{}

func (*duplicateErr) Error() string { return "duplicate key value violates unique constraint" }

func (r *memPaymentRepo) FindPaymentByID(_ context.Context, id string) (shareddomain.Payment, error) {
	if p, ok := r.payments[id]; ok {
		return p, nil
	}
	return shareddomain.Payment{}, gorm.ErrRecordNotFound
}

func (r *memPaymentRepo) FindPaymentByToken(_ context.Context, token string) (shareddomain.Payment, error) {
	for _, p := range r.payments {
		if p.Token == token {
			return p, nil
		}
	}
	return shareddomain.Payment{}, gorm.ErrRecordNotFound
}

func (r *memPaymentRepo) FindActivePaymentByRef(_ context.Context, source, ref string) (shareddomain.Payment, error) {
	for _, p := range r.payments {
		if p.Source == source && p.ReferenceID == ref && (p.Status == shareddomain.PaymentPending || p.Status == shareddomain.PaymentProcessing) {
			return p, nil
		}
	}
	return shareddomain.Payment{}, gorm.ErrRecordNotFound
}

func (r *memPaymentRepo) LockPayment(ctx context.Context, id string) (shareddomain.Payment, error) {
	return r.FindPaymentByID(ctx, id)
}

func (r *memPaymentRepo) FetchAllPayments(context.Context, *domain.FilterPayment) ([]shareddomain.Payment, error) {
	var out []shareddomain.Payment
	for _, p := range r.payments {
		out = append(out, p)
	}
	return out, nil
}

func (r *memPaymentRepo) CountPayments(context.Context, *domain.FilterPayment) int {
	return len(r.payments)
}

func (r *memPaymentRepo) FetchExpiredPaymentIDs(_ context.Context, now time.Time, limit int) ([]string, error) {
	var ids []string
	for _, p := range r.payments {
		if (p.Status == shareddomain.PaymentPending || p.Status == shareddomain.PaymentProcessing) && p.ExpiresAt.Before(now) {
			ids = append(ids, p.ID)
		}
	}
	sort.Strings(ids)
	if len(ids) > limit {
		ids = ids[:limit]
	}
	return ids, nil
}

func (r *memPaymentRepo) SaveTransaction(_ context.Context, d *shareddomain.Transaction) error {
	if d.CashCode != nil {
		for id, t := range r.txns {
			if id != d.ID && t.CashCode != nil && *t.CashCode == *d.CashCode {
				return &duplicateErr{}
			}
		}
	}
	if d.CreatedAt.IsZero() {
		d.CreatedAt = time.Now()
		r.seq++
		r.created[d.ID] = r.seq
	}
	r.txns[d.ID] = *d
	return nil
}

func (r *memPaymentRepo) FindTransactionByID(_ context.Context, id string) (shareddomain.Transaction, error) {
	if t, ok := r.txns[id]; ok {
		return t, nil
	}
	return shareddomain.Transaction{}, gorm.ErrRecordNotFound
}

func (r *memPaymentRepo) FindTransactionByCashCode(_ context.Context, code string) (shareddomain.Transaction, error) {
	for _, t := range r.txns {
		if t.CashCode != nil && *t.CashCode == code {
			return t, nil
		}
	}
	return shareddomain.Transaction{}, gorm.ErrRecordNotFound
}

func (r *memPaymentRepo) FetchTransactionsByPayment(_ context.Context, paymentID string) ([]shareddomain.Transaction, error) {
	var out []shareddomain.Transaction
	for _, t := range r.txns {
		if t.PaymentID == paymentID {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return r.created[out[i].ID] < r.created[out[j].ID] })
	return out, nil
}

func (r *memPaymentRepo) FindOpenTransaction(ctx context.Context, paymentID string) (shareddomain.Transaction, error) {
	txns, _ := r.FetchTransactionsByPayment(ctx, paymentID)
	for i := len(txns) - 1; i >= 0; i-- {
		if txns[i].Status == shareddomain.TransactionPending {
			return txns[i], nil
		}
	}
	return shareddomain.Transaction{}, gorm.ErrRecordNotFound
}

func (r *memPaymentRepo) SaveCallbackLog(_ context.Context, d *shareddomain.CallbackLog) error {
	if d.ID == 0 {
		r.nextLog++
		d.ID = r.nextLog
		r.logs = append(r.logs, *d)
		return nil
	}
	for i := range r.logs {
		if r.logs[i].ID == d.ID {
			r.logs[i] = *d
		}
	}
	return nil
}

func (r *memPaymentRepo) FindCallbackLogByID(_ context.Context, id int) (shareddomain.CallbackLog, error) {
	for _, l := range r.logs {
		if l.ID == id {
			return l, nil
		}
	}
	return shareddomain.CallbackLog{}, gorm.ErrRecordNotFound
}

func (r *memPaymentRepo) FetchAllCallbackLogs(context.Context, *domain.FilterCallbackLog) ([]shareddomain.CallbackLog, error) {
	return r.logs, nil
}

func (r *memPaymentRepo) CountCallbackLogs(context.Context, *domain.FilterCallbackLog) int {
	return len(r.logs)
}

func (r *memPaymentRepo) SaveOutbox(_ context.Context, d *shareddomain.Outbox) error {
	if d.ID == 0 {
		r.nextOut++
		d.ID = r.nextOut
		r.outbox = append(r.outbox, *d)
		return nil
	}
	for i := range r.outbox {
		if r.outbox[i].ID == d.ID {
			r.outbox[i] = *d
		}
	}
	return nil
}

func (r *memPaymentRepo) LockPendingOutbox(_ context.Context, limit int) ([]shareddomain.Outbox, error) {
	var out []shareddomain.Outbox
	for _, o := range r.outbox {
		if o.PublishedAt == nil && len(out) < limit {
			out = append(out, o)
		}
	}
	return out, nil
}

// events returns the outbox rows of an event type, oldest first
func (r *memPaymentRepo) events(eventType string) []shareddomain.Outbox {
	var out []shareddomain.Outbox
	for _, o := range r.outbox {
		if o.EventType == eventType {
			out = append(out, o)
		}
	}
	return out
}

type memMethodRepo struct{ methods []shareddomain.Method }

func (r *memMethodRepo) FetchAll(context.Context, *methoddomain.FilterMethod) ([]shareddomain.Method, error) {
	return r.methods, nil
}
func (r *memMethodRepo) Count(context.Context, *methoddomain.FilterMethod) int { return len(r.methods) }
func (r *memMethodRepo) FindByID(_ context.Context, id int) (shareddomain.Method, error) {
	for _, m := range r.methods {
		if m.ID == id {
			return m, nil
		}
	}
	return shareddomain.Method{}, gorm.ErrRecordNotFound
}
func (r *memMethodRepo) FindByCode(_ context.Context, code string) (shareddomain.Method, error) {
	for _, m := range r.methods {
		if m.Code == code {
			return m, nil
		}
	}
	return shareddomain.Method{}, gorm.ErrRecordNotFound
}
func (r *memMethodRepo) FetchEnabled(context.Context) ([]shareddomain.Method, error) {
	var out []shareddomain.Method
	for _, m := range r.methods {
		if m.IsEnabled {
			out = append(out, m)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].SortOrder < out[j].SortOrder })
	return out, nil
}
func (r *memMethodRepo) Save(_ context.Context, d *shareddomain.Method) error {
	for i := range r.methods {
		if r.methods[i].Code == d.Code && r.methods[i].ID != d.ID {
			return &duplicateErr{}
		}
	}
	if d.ID == 0 {
		d.ID = len(r.methods) + 1
		r.methods = append(r.methods, *d)
		return nil
	}
	for i := range r.methods {
		if r.methods[i].ID == d.ID {
			r.methods[i] = *d
		}
	}
	return nil
}
func (r *memMethodRepo) Delete(_ context.Context, id int) error {
	for i := range r.methods {
		if r.methods[i].ID == id {
			r.methods = append(r.methods[:i], r.methods[i+1:]...)
			break
		}
	}
	return nil
}

type memGatewayRepo struct {
	gateways map[string]shareddomain.Gateway
}

func (r *memGatewayRepo) FetchAll(context.Context) ([]shareddomain.Gateway, error) {
	var out []shareddomain.Gateway
	for _, g := range r.gateways {
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out, nil
}
func (r *memGatewayRepo) FindByCode(_ context.Context, code string) (shareddomain.Gateway, error) {
	if g, ok := r.gateways[code]; ok {
		return g, nil
	}
	return shareddomain.Gateway{}, gorm.ErrRecordNotFound
}
func (r *memGatewayRepo) Save(_ context.Context, d *shareddomain.Gateway) error {
	r.gateways[d.Code] = *d
	return nil
}

type memTopicRepo struct {
	topics []shareddomain.Topic
	gw     *memGatewayRepo
}

func (r *memTopicRepo) FetchAll(context.Context, *topicdomain.FilterTopic) ([]shareddomain.Topic, error) {
	return r.topics, nil
}
func (r *memTopicRepo) Count(context.Context, *topicdomain.FilterTopic) int { return len(r.topics) }
func (r *memTopicRepo) FindByID(_ context.Context, id int) (shareddomain.Topic, error) {
	for _, t := range r.topics {
		if t.ID == id {
			return t, nil
		}
	}
	return shareddomain.Topic{}, gorm.ErrRecordNotFound
}
func (r *memTopicRepo) Save(_ context.Context, d *shareddomain.Topic) error {
	if d.ID == 0 {
		d.ID = len(r.topics) + 1
		r.topics = append(r.topics, *d)
		return nil
	}
	for i := range r.topics {
		if r.topics[i].ID == d.ID {
			r.topics[i] = *d
		}
	}
	return nil
}
func (r *memTopicRepo) Delete(context.Context, int) error { return nil }
func (r *memTopicRepo) FetchEnabledConsume(ctx context.Context) ([]shareddomain.Topic, error) {
	var out []shareddomain.Topic
	for _, t := range r.topics {
		if t.Direction != shareddomain.TopicConsume || !t.IsEnabled || t.GatewayCode == nil {
			continue
		}
		if g, err := r.gw.FindByCode(ctx, *t.GatewayCode); err == nil && g.IsEnabled {
			out = append(out, t)
		}
	}
	return out, nil
}
func (r *memTopicRepo) FetchEnabledPublish(_ context.Context, eventType string) ([]shareddomain.Topic, error) {
	var out []shareddomain.Topic
	for _, t := range r.topics {
		if t.Direction == shareddomain.TopicPublish && t.IsEnabled && t.EventType != nil && *t.EventType == eventType {
			out = append(out, t)
		}
	}
	return out, nil
}
func (r *memTopicRepo) FindConsumeByTopic(_ context.Context, topic string) (shareddomain.Topic, error) {
	for _, t := range r.topics {
		if t.Direction == shareddomain.TopicConsume && t.Topic == topic {
			return t, nil
		}
	}
	return shareddomain.Topic{}, gorm.ErrRecordNotFound
}

// memRepoSQL is the repository.RepoSQL the usecase talks to
type memRepoSQL struct {
	mu      sync.Mutex
	pay     *memPaymentRepo
	method  *memMethodRepo
	gateway *memGatewayRepo
	topic   *memTopicRepo
}

var _ repository.RepoSQL = (*memRepoSQL)(nil)

func (r *memRepoSQL) PaymentRepo() paymentrepo.PaymentRepository { return r.pay }
func (r *memRepoSQL) MethodRepo() methodrepo.MethodRepository    { return r.method }
func (r *memRepoSQL) GatewayRepo() gatewayrepo.GatewayRepository { return r.gateway }
func (r *memRepoSQL) TopicRepo() topicrepo.TopicRepository       { return r.topic }

// WithTransaction runs fn and restores the payment data if it fails, like a DB rollback
func (r *memRepoSQL) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	snapshot := r.pay.clone()
	if err := fn(ctx); err != nil {
		*r.pay = *snapshot
		return err
	}
	return nil
}

// ---- fake gateway provider

type fakeProvider struct {
	code        string
	charges     []provider.ChargeRequest
	chargeErr   error
	cancelled   []string
	callback    provider.CallbackResult
	callbackErr error
}

func (f *fakeProvider) Code() string { return f.code }
func (f *fakeProvider) CreateCharge(_ context.Context, _ provider.Config, req provider.ChargeRequest) (provider.ChargeResult, error) {
	f.charges = append(f.charges, req)
	if f.chargeErr != nil {
		return provider.ChargeResult{Raw: []byte(`{"status":"rejected"}`)}, f.chargeErr
	}
	return provider.ChargeResult{
		Instruction: map[string]any{"type": req.Method.Type, "bank": "bca", "vaNumber": "123456", "channel": req.Method.Channel},
		ExternalRef: "ext-" + req.TransactionID, Raw: []byte(`{"ok":true}`),
	}, nil
}
func (f *fakeProvider) Cancel(_ context.Context, _ provider.Config, txID, _ string) error {
	f.cancelled = append(f.cancelled, txID)
	return nil
}
func (f *fakeProvider) ParseCallback(context.Context, provider.Config, map[string]string, []byte) (provider.CallbackResult, error) {
	return f.callback, f.callbackErr
}

// ---- publisher

type fakePublisher struct {
	msgs []candishared.PublisherArgument
	err  error
}

func (p *fakePublisher) PublishMessage(_ context.Context, a *candishared.PublisherArgument) error {
	if p.err != nil {
		return p.err
	}
	p.msgs = append(p.msgs, *a)
	return nil
}

func (p *fakePublisher) topics() (out []string) {
	for _, m := range p.msgs {
		out = append(out, m.Topic)
	}
	return
}

// ---- harness

type harness struct {
	uc    *paymentUsecaseImpl
	repo  *memRepoSQL
	prov  *fakeProvider
	pub   *fakePublisher
	now   time.Time
	env   shared.Environment
	kicks int
}

func strp(s string) *string { return &s }

func newHarness(t *testing.T) *harness {
	t.Helper()
	gw := &memGatewayRepo{gateways: map[string]shareddomain.Gateway{
		"midtrans": {Code: "midtrans", IsEnabled: true, Environment: "sandbox", CredentialsEnc: "enc"},
		"xendit":   {Code: "xendit", IsEnabled: false, CredentialsEnc: "enc"},
		"mock":     {Code: "mock", IsEnabled: true, Environment: "sandbox"},
	}}
	repo := &memRepoSQL{
		pay: newMemPaymentRepo(),
		method: &memMethodRepo{methods: []shareddomain.Method{
			{ID: 1, Code: "cash", Name: "Cash", Type: "cash", IsEnabled: true, SortOrder: 1, Instructions: "Pay at the counter"},
			{ID: 2, Code: "bca_va", Name: "BCA VA", Type: "virtual_account", GatewayCode: strp("midtrans"), GatewayChannel: "bca", IsEnabled: true, SortOrder: 10, FeeFlat: 1000},
			{ID: 3, Code: "qris", Name: "QRIS", Type: "qris", GatewayCode: strp("midtrans"), GatewayChannel: "qris", IsEnabled: true, SortOrder: 20, FeePercent: 0.7, MinAmount: 1000, MaxAmount: 5000000},
			{ID: 4, Code: "xendit_invoice", Name: "Xendit", Type: "virtual_account", GatewayCode: strp("xendit"), GatewayChannel: "invoice", IsEnabled: true, SortOrder: 30},
			{ID: 5, Code: "old_method", Name: "Disabled", Type: "cash", IsEnabled: false, SortOrder: 99},
		}},
		gateway: gw,
	}
	repo.topic = &memTopicRepo{gw: gw, topics: []shareddomain.Topic{
		{ID: 1, Topic: "payment.midtrans_callback_received", Direction: "consume", GatewayCode: strp("midtrans"), IsEnabled: true},
		{ID: 2, Topic: "payment.xendit_callback_received", Direction: "consume", GatewayCode: strp("xendit"), IsEnabled: true},
		{ID: 3, Topic: "payment.mock_callback_received", Direction: "consume", GatewayCode: strp("mock"), IsEnabled: true},
		{ID: 10, Topic: "payment.created", Direction: "publish", EventType: strp("payment.created"), IsEnabled: true},
		{ID: 11, Topic: "payment.checkout_started", Direction: "publish", EventType: strp("payment.checkout_started"), IsEnabled: true},
		{ID: 12, Topic: "payment.completed", Direction: "publish", EventType: strp("payment.completed"), IsEnabled: true},
		{ID: 13, Topic: "payment.expired", Direction: "publish", EventType: strp("payment.expired"), IsEnabled: true},
		{ID: 14, Topic: "payment.cancelled", Direction: "publish", EventType: strp("payment.cancelled"), IsEnabled: true},
		{ID: 15, Topic: "notification.requested", Direction: "publish", EventType: strp("notification_email"), IsEnabled: true},
	}}

	h := &harness{
		repo: repo, prov: &fakeProvider{code: "midtrans"}, pub: &fakePublisher{},
		now: time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC),
		env: shared.Environment{CheckoutBaseURL: "https://pay.example.com/pay", DefaultPaymentExpiry: 24 * time.Hour, GatewayEncryptionSecret: "secret"},
	}
	var n int
	h.uc = &paymentUsecaseImpl{
		repoSQL:     repo,
		now:         func() time.Time { return h.now },
		newID:       newUUID,
		newToken:    func() string { n++; return fmt.Sprintf("token-%d", n) },
		newCashCode: func() string { n++; return fmt.Sprintf("CASH%04d", n) },
		env:         func() shared.Environment { return h.env },
		publisher:   func() interfaces.Publisher { return h.pub },
		providers: func(row shareddomain.Gateway, requireEnabled bool) (provider.Provider, provider.Config, error) {
			if requireEnabled && !row.IsEnabled {
				return nil, provider.Config{}, fmt.Errorf("gateway %s is disabled", row.Code)
			}
			return h.prov, provider.Config{Code: row.Code}, nil
		},
		afterCommit: func() { h.kicks++ },
	}
	return h
}

func (h *harness) create(t *testing.T, ref string, amount int64) domain.ResponseCreatePayment {
	t.Helper()
	res, err := h.uc.CreatePayment(context.Background(), "order", &domain.RequestCreatePayment{
		ReferenceID: ref, Amount: amount, Description: "Order " + ref,
		Customer: shareddomain.Customer{Name: "Budi", Email: "budi@example.com"},
		Items:    []shareddomain.Item{{Name: "Nasi", Price: amount, Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	return res
}
