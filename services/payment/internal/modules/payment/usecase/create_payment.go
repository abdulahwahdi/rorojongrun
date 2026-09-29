package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"monorepo/services/payment/internal/modules/payment/domain"
	"monorepo/services/payment/pkg/helper"
	shareddomain "monorepo/services/payment/pkg/shared/domain"

	"github.com/golangid/candi/tracer"
	"gorm.io/gorm"
)

// CreatePayment issues a payment link for another service. It is idempotent per
// (source, referenceId): while a payment of that reference is still open the same link is returned.
func (uc *paymentUsecaseImpl) CreatePayment(ctx context.Context, source string, req *domain.RequestCreatePayment) (res domain.ResponseCreatePayment, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PaymentUsecase:CreatePayment")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	if err = validateCreate(source, req); err != nil {
		return res, err
	}
	repo := uc.repoSQL.PaymentRepo()

	if existing, ferr := repo.FindActivePaymentByRef(ctx, source, req.ReferenceID); ferr == nil {
		if uc.now().After(existing.ExpiresAt) {
			// the open payment is overdue but the cron has not swept it yet
			if err = uc.expirePayment(ctx, existing.ID); err != nil {
				return res, err
			}
		} else {
			if existing.Amount != req.Amount {
				return res, helper.NewConflict("an open payment for this reference exists with a different amount")
			}
			return uc.createResponse(&existing), nil
		}
	} else if !errors.Is(ferr, gorm.ErrRecordNotFound) {
		return res, ferr
	}

	expiry := uc.env().DefaultPaymentExpiry
	if req.ExpiresInSec > 0 {
		expiry = time.Duration(req.ExpiresInSec) * time.Second
	}
	currency := req.Currency
	if currency == "" {
		currency = "IDR"
	}
	items := req.Items
	if items == nil {
		items = []shareddomain.Item{}
	}
	allowed := req.AllowedMethods
	if allowed == nil {
		allowed = []string{}
	}
	p := shareddomain.Payment{
		ID: uc.newID(), Token: uc.newToken(), Source: source, ReferenceID: req.ReferenceID,
		Description: req.Description, Amount: req.Amount, TotalAmount: req.Amount, Currency: currency,
		Status: shareddomain.PaymentPending, Customer: shareddomain.NewJSON(req.Customer),
		Items: shareddomain.NewJSON(items), Metadata: shareddomain.NewJSON(req.Metadata),
		AllowedMethods: shareddomain.NewJSON(allowed), SuccessURL: req.SuccessURL, FailureURL: req.FailureURL,
		ExpiresAt: uc.now().Add(expiry),
	}

	err = uc.repoSQL.WithTransaction(ctx, func(ctx context.Context) error {
		if err := repo.SavePayment(ctx, &p); err != nil {
			return err
		}
		return uc.enqueueEvent(ctx, shareddomain.EventPaymentCreated, &p, nil)
	})
	if err != nil {
		// lost a race with a concurrent create of the same reference: hand out the winner's link
		if existing, ferr := repo.FindActivePaymentByRef(ctx, source, req.ReferenceID); ferr == nil {
			return uc.createResponse(&existing), nil
		}
		return res, helper.MapDBError(err)
	}
	uc.afterCommit()
	return uc.createResponse(&p), nil
}

func validateCreate(source string, req *domain.RequestCreatePayment) error {
	if source == "" {
		return helper.NewInvalid("source is required")
	}
	if strings.TrimSpace(req.ReferenceID) == "" {
		return helper.NewInvalid("referenceId is required")
	}
	if req.Amount <= 0 {
		return helper.NewInvalid("amount must be greater than zero")
	}
	if req.Currency != "" && req.Currency != "IDR" {
		return helper.NewInvalid("only IDR is supported")
	}
	if e := req.Customer.Email; e != "" && !strings.Contains(e, "@") {
		return helper.NewInvalid("customer.email is not a valid email address")
	}
	if req.ExpiresInSec != 0 && (req.ExpiresInSec < domain.MinExpirySec || req.ExpiresInSec > domain.MaxExpirySec) {
		return helper.NewInvalid("expiresInSec must be between 60 and 2592000")
	}
	for _, it := range req.Items {
		if it.Quantity <= 0 || it.Price < 0 {
			return helper.NewInvalid("every item needs a positive quantity and a non-negative price")
		}
	}
	return nil
}

func (uc *paymentUsecaseImpl) createResponse(p *shareddomain.Payment) domain.ResponseCreatePayment {
	return domain.ResponseCreatePayment{
		PaymentID: p.ID, Token: p.Token, PaymentURL: uc.paymentURL(p), Status: p.Status,
		ReferenceID: p.ReferenceID, Amount: p.Amount, Currency: p.Currency, ExpiresAt: p.ExpiresAt.Format(time.RFC3339),
	}
}
