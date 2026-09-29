package usecase

import (
	"context"
	"errors"
	"strings"

	"monorepo/sdk/notification"
	"monorepo/services/user/internal/modules/auth/domain"
	"monorepo/services/user/pkg/helper"
	shareddomain "monorepo/services/user/pkg/shared/domain"
	"monorepo/services/user/pkg/shared/usecase/common"

	"github.com/golangid/candi/logger"
	"github.com/golangid/candi/tracer"
	"gorm.io/gorm"
)

func otpVerifyRequest(email, realm, code string) notification.VerifyOTPRequest {
	return notification.VerifyOTPRequest{Recipient: email, Purpose: domain.OTPPurposePrefix + realm, Code: code}
}

func (uc *authUsecaseImpl) RequestOTP(ctx context.Context, realmName string, req *domain.RequestOTPLogin) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "AuthUsecase:RequestOTP")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	realm, err := common.LoadRealm(ctx, uc.repoSQL, realmName)
	if err != nil {
		return err
	}
	if !realm.Enabled || !realm.OTPLoginEnabled {
		return helper.NewForbidden("otp login is not enabled for this realm")
	}
	client, err := uc.repoSQL.ClientRepo().FindByClientID(ctx, realm.ID, req.ClientID)
	if err != nil || !client.Enabled || client.Type == shareddomain.ClientTypeConfidential || !grantAllowed(client, domain.GrantOTP) {
		// confidential clients must present their secret, so they cannot start an unauthenticated OTP flow
		return helper.NewUnauthorized("invalid client")
	}
	notifier := uc.notification()
	if notifier == nil {
		return helper.NewInvalid("otp login is not configured")
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	user, err := uc.repoSQL.UserRepo().FindByEmail(ctx, realm.ID, email)
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && (user.IsServiceAccount || user.Status == shareddomain.UserStatusDisabled)) {
		return nil // same answer as for a known account
	}
	if err != nil {
		return err
	}
	if _, sendErr := notifier.RequestOTP(ctx, notification.RequestOTPRequest{
		Recipient: email, Channel: "email", Purpose: domain.OTPPurposePrefix + realm.Name,
	}); sendErr != nil {
		// do not leak delivery problems of existing accounts to the caller
		logger.LogE("auth: request otp failed: " + sendErr.Error())
	}
	return nil
}
