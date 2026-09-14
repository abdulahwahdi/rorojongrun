package usecase

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"monorepo/services/notification/internal/modules/otp/domain"
	notificationdomain "monorepo/services/notification/internal/modules/notification/domain"
	shareddomain "monorepo/services/notification/pkg/shared/domain"

	"github.com/golangid/candi/tracer"
	"golang.org/x/crypto/bcrypt"
)

// RequestOTP generates a 6-digit code, stores it hashed with an expiry, and
// asks the notification module (via the shared-usecase cross-module call)
// to deliver it. bcrypt is used rather than a fast hash (e.g. SHA-256)
// because the code's keyspace is only 10^6 — a fast hash would make a
// stolen code_hash trivially brute-forceable within the expiry window;
// bcrypt's cost factor meaningfully slows that down, and this isn't a hot
// path so the extra ~50-100ms per request is fine.
func (uc *otpUsecaseImpl) RequestOTP(ctx context.Context, req *domain.RequestOTP) (jobID string, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "OtpUsecase:RequestOTP")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	code, err := generateNumericCode(6)
	if err != nil {
		return "", err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(code), 10)
	if err != nil {
		return "", err
	}

	otpReq := shareddomain.OTPRequest{
		Recipient:    req.Recipient,
		Channel:      req.Channel,
		Purpose:      req.Purpose,
		CodeHash:     string(hash),
		ExpiresAt:    time.Now().Add(domain.DefaultExpiry),
		MaxAttempts:  domain.MaxAttempts,
		AttemptCount: 0,
	}
	if err = uc.repoSQL.OtpRepo().Save(ctx, &otpReq); err != nil {
		return "", err
	}

	jobID, err = uc.sharedUsecase.SendNotification(ctx, &notificationdomain.RequestSendNotification{
		Channel:      req.Channel,
		TemplateCode: domain.TemplateCodeOTPVerification,
		Recipient:    req.Recipient,
		Variables: map[string]any{
			"code":    code,
			"purpose": req.Purpose,
		},
	})
	return
}

func generateNumericCode(digits int) (string, error) {
	max := big.NewInt(1)
	for i := 0; i < digits; i++ {
		max.Mul(max, big.NewInt(10))
	}
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", digits, n), nil
}
