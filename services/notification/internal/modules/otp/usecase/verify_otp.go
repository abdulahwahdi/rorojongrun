package usecase

import (
	"context"
	"time"

	"monorepo/services/notification/internal/modules/otp/domain"

	"github.com/golangid/candi/tracer"
	"golang.org/x/crypto/bcrypt"
)

// VerifyOTP checks a submitted code against the latest unverified request
// for recipient+purpose.
func (uc *otpUsecaseImpl) VerifyOTP(ctx context.Context, req *domain.RequestVerifyOTP) (verified bool, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "OtpUsecase:VerifyOTP")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	otpReq, err := uc.repoSQL.OtpRepo().FindLatestUnverified(ctx, req.Recipient, req.Purpose)
	if err != nil {
		return false, domain.ErrOTPNotFound
	}

	if time.Now().After(otpReq.ExpiresAt) {
		return false, domain.ErrOTPExpired
	}
	if otpReq.AttemptCount >= otpReq.MaxAttempts {
		return false, domain.ErrOTPTooManyAttempts
	}

	if bcrypt.CompareHashAndPassword([]byte(otpReq.CodeHash), []byte(req.Code)) != nil {
		_ = uc.repoSQL.OtpRepo().IncrementAttempt(ctx, otpReq.ID)
		return false, domain.ErrOTPMismatch
	}

	if err = uc.repoSQL.OtpRepo().MarkVerified(ctx, otpReq.ID); err != nil {
		return false, err
	}
	return true, nil
}
