package usecase

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"monorepo/globalshared/rest"
	"monorepo/services/order/internal/modules/merchant/domain"
	shareddomain "monorepo/services/order/pkg/shared/domain"
	"monorepo/services/order/pkg/shared/usecase/common"

	"github.com/golangid/candi/candishared"
	"github.com/golangid/candi/tracer"
	"gorm.io/gorm"
)

var prefixPattern = regexp.MustCompile(`^[A-Z0-9]{1,10}$`)

func (uc *merchantUsecaseImpl) GetAllMerchants(ctx context.Context, filter *domain.FilterMerchant) (res domain.ResponseMerchantList, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "MerchantUsecase:GetAllMerchants")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	if res.Data, err = uc.repoSQL.MerchantRepo().FetchAll(ctx, filter); err != nil {
		return
	}
	res.Meta = candishared.NewMeta(filter.Page, filter.Limit, uc.repoSQL.MerchantRepo().Count(ctx, filter))
	return
}

func (uc *merchantUsecaseImpl) GetMerchant(ctx context.Context, merchantID string) (res shareddomain.MerchantSetting, err error) {
	res, err = uc.repoSQL.MerchantRepo().Find(ctx, merchantID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = rest.NewNotFound("merchant settings not found")
	}
	return
}

func (uc *merchantUsecaseImpl) SaveMerchant(ctx context.Context, merchantID string, req *domain.RequestSaveMerchant) (res shareddomain.MerchantSetting, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "MerchantUsecase:SaveMerchant")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	res = shareddomain.MerchantSetting{
		MerchantID: strings.TrimSpace(merchantID), Name: req.Name, Address: req.Address, TaxID: req.TaxID,
		OrderPrefix: strings.ToUpper(req.OrderPrefix), InvoicePrefix: strings.ToUpper(req.InvoicePrefix),
		CreditNotePrefix: strings.ToUpper(req.CreditNotePrefix), TaxName: req.TaxName, TaxRate: req.TaxRate,
		TaxMode: req.TaxMode, RoundingMode: req.RoundingMode, RoundingUnit: req.RoundingUnit, Timezone: req.Timezone,
	}
	withSettingDefaults(&res)
	if err = validateSetting(res); err != nil {
		return res, err
	}
	if existing, ferr := uc.repoSQL.MerchantRepo().Find(ctx, res.MerchantID); ferr == nil {
		res.CreatedAt = existing.CreatedAt
	}
	err = uc.repoSQL.MerchantRepo().Save(ctx, &res)
	return res, rest.MapDBError(err)
}

func withSettingDefaults(m *shareddomain.MerchantSetting) {
	def := func(v *string, d string) {
		if strings.TrimSpace(*v) == "" {
			*v = d
		}
	}
	def(&m.OrderPrefix, "ORD")
	def(&m.InvoicePrefix, "INV")
	def(&m.CreditNotePrefix, "CN")
	def(&m.TaxName, "PPN")
	def(&m.TaxMode, shareddomain.TaxNone)
	def(&m.RoundingMode, shareddomain.RoundNone)
	def(&m.Timezone, "Asia/Jakarta")
	if m.RoundingUnit < 1 {
		m.RoundingUnit = 1
	}
}

func validateSetting(m shareddomain.MerchantSetting) error {
	if m.MerchantID == "" || strings.ContainsAny(m.MerchantID, "/ ") {
		return rest.NewInvalid("merchantId must not be empty nor contain '/' or spaces")
	}
	for _, p := range []string{m.OrderPrefix, m.InvoicePrefix, m.CreditNotePrefix} {
		if !prefixPattern.MatchString(p) {
			return rest.NewInvalid("prefixes are 1 to 10 letters or digits: " + p)
		}
	}
	if m.InvoicePrefix == m.CreditNotePrefix {
		return rest.NewInvalid("invoicePrefix and creditNotePrefix must differ")
	}
	if m.TaxRate < 0 || m.TaxRate > 100 {
		return rest.NewInvalid("taxRate is a percent between 0 and 100")
	}
	if _, err := time.LoadLocation(m.Timezone); err != nil {
		return rest.NewInvalid("unknown timezone: " + m.Timezone)
	}
	return nil
}

func (uc *merchantUsecaseImpl) DeleteMerchant(ctx context.Context, merchantID string) error {
	if merchantID == shareddomain.DefaultMerchant {
		return rest.NewConflict("the default merchant settings cannot be deleted")
	}
	deleted, err := uc.repoSQL.MerchantRepo().Delete(ctx, merchantID)
	if err == nil && !deleted {
		err = rest.NewNotFound("merchant settings not found")
	}
	return err
}

func (uc *merchantUsecaseImpl) Quote(ctx context.Context, merchantID string, lines []shareddomain.PriceLine) (res shareddomain.Breakdown, err error) {
	m, err := uc.MerchantSettings(ctx, merchantID)
	if err != nil {
		return res, err
	}
	return common.Price(lines, m), nil
}

func (uc *merchantUsecaseImpl) MerchantSettings(ctx context.Context, merchantID string) (m shareddomain.MerchantSetting, err error) {
	if merchantID == "" {
		merchantID = shareddomain.DefaultMerchant
	}
	m, err = uc.repoSQL.MerchantRepo().Find(ctx, merchantID)
	if errors.Is(err, gorm.ErrRecordNotFound) && merchantID != shareddomain.DefaultMerchant {
		// the default settings, numbered in the merchant's own sequences
		m, err = uc.repoSQL.MerchantRepo().Find(ctx, shareddomain.DefaultMerchant)
		m.MerchantID = merchantID
	}
	if errors.Is(err, gorm.ErrRecordNotFound) { // the migration seeds '*'; keep working without it
		m, err = shareddomain.MerchantSetting{MerchantID: merchantID}, nil
	}
	if err != nil {
		return m, err
	}
	withSettingDefaults(&m)
	return m, nil
}

func (uc *merchantUsecaseImpl) NextOrderNumber(ctx context.Context, m shareddomain.MerchantSetting, at time.Time) (string, error) {
	period := common.OrderNumberPeriod(m, at)
	n, err := uc.repoSQL.SequenceRepo().Next(ctx, common.OrderNumberScope(m.OrderPrefix), period)
	if err != nil {
		return "", err
	}
	return common.FormatOrderNumber(m, period, n), nil
}

func (uc *merchantUsecaseImpl) NextInvoiceNumber(ctx context.Context, m shareddomain.MerchantSetting, invoiceType string, at time.Time) (string, error) {
	prefix := m.InvoicePrefix
	if invoiceType == shareddomain.InvoiceTypeCreditNote {
		prefix = m.CreditNotePrefix
	}
	period := common.InvoiceNumberPeriod(m, at)
	n, err := uc.repoSQL.SequenceRepo().Next(ctx, common.InvoiceNumberScope(invoiceType, m.MerchantID), period)
	if err != nil {
		return "", err
	}
	return common.FormatInvoiceNumber(prefix, m.MerchantID, period, n), nil
}
