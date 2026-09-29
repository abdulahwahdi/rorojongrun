package usecase

import (
	"context"
	"errors"
	"regexp"

	"monorepo/services/payment/internal/modules/method/domain"
	"monorepo/services/payment/pkg/helper"
	shareddomain "monorepo/services/payment/pkg/shared/domain"

	"github.com/golangid/candi/tracer"
	"gorm.io/gorm"
)

var methodCodeRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_]{1,49}$`)

var methodTypes = map[string]bool{
	shareddomain.MethodVirtualAccount: true, shareddomain.MethodQRIS: true, shareddomain.MethodEWallet: true,
	shareddomain.MethodCard: true, shareddomain.MethodRetail: true, shareddomain.MethodCash: true,
}

// validate checks business rules the JSON schema cannot: the type/gateway pairing
func (uc *methodUsecaseImpl) validate(ctx context.Context, req *domain.RequestSaveMethod) error {
	if !methodCodeRe.MatchString(req.Code) {
		return helper.NewInvalid("code must be 2-50 chars of a-z, 0-9 and underscore")
	}
	if !methodTypes[req.Type] {
		return helper.NewInvalid("unknown method type " + req.Type)
	}
	if req.MinAmount < 0 || req.MaxAmount < 0 || req.FeeFlat < 0 || req.FeePercent < 0 || req.FeePercent > 100 {
		return helper.NewInvalid("amounts and fees cannot be negative, feePercent is 0-100")
	}
	if req.MaxAmount != 0 && req.MaxAmount < req.MinAmount {
		return helper.NewInvalid("maxAmount must be 0 (no limit) or >= minAmount")
	}
	if req.Type == shareddomain.MethodCash {
		if req.GatewayCode != "" {
			return helper.NewInvalid("a cash method has no gateway")
		}
		return nil
	}
	if req.GatewayCode == "" || req.GatewayChannel == "" {
		return helper.NewInvalid("gatewayCode and gatewayChannel are required for a non-cash method")
	}
	if _, err := uc.repoSQL.GatewayRepo().FindByCode(ctx, req.GatewayCode); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return helper.NewInvalid("unknown gateway " + req.GatewayCode)
		}
		return err
	}
	return nil
}

func apply(m *shareddomain.Method, req *domain.RequestSaveMethod) {
	m.Code, m.Name, m.Type = req.Code, req.Name, req.Type
	m.GatewayCode, m.GatewayChannel = helper.StrPtr(req.GatewayCode), req.GatewayChannel
	m.IsEnabled, m.IconURL, m.SortOrder = req.IsEnabled, req.IconURL, req.SortOrder
	m.MinAmount, m.MaxAmount, m.FeeFlat, m.FeePercent = req.MinAmount, req.MaxAmount, req.FeeFlat, req.FeePercent
	m.Instructions = req.Instructions
}

func (uc *methodUsecaseImpl) CreateMethod(ctx context.Context, req *domain.RequestSaveMethod) (data shareddomain.Method, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "MethodUsecase:CreateMethod")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	if err = uc.validate(ctx, req); err != nil {
		return data, err
	}
	apply(&data, req)
	if err = uc.repoSQL.MethodRepo().Save(ctx, &data); err != nil {
		return shareddomain.Method{}, helper.MapDBError(err)
	}
	return data, nil
}

func (uc *methodUsecaseImpl) UpdateMethod(ctx context.Context, id int, req *domain.RequestSaveMethod) (data shareddomain.Method, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "MethodUsecase:UpdateMethod")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	if data, err = uc.GetMethod(ctx, id); err != nil {
		return data, err
	}
	if err = uc.validate(ctx, req); err != nil {
		return data, err
	}
	apply(&data, req)
	if err = uc.repoSQL.MethodRepo().Save(ctx, &data); err != nil {
		return shareddomain.Method{}, helper.MapDBError(err)
	}
	return data, nil
}

func (uc *methodUsecaseImpl) SetMethodStatus(ctx context.Context, id int, enabled bool) (data shareddomain.Method, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "MethodUsecase:SetMethodStatus")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	if data, err = uc.GetMethod(ctx, id); err != nil {
		return data, err
	}
	data.IsEnabled = enabled
	err = uc.repoSQL.MethodRepo().Save(ctx, &data)
	return data, err
}

func (uc *methodUsecaseImpl) DeleteMethod(ctx context.Context, id int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "MethodUsecase:DeleteMethod")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	if _, err = uc.GetMethod(ctx, id); err != nil {
		return err
	}
	return uc.repoSQL.MethodRepo().Delete(ctx, id)
}
