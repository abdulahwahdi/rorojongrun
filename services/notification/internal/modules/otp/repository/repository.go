package repository

import (
	"context"

	shareddomain "monorepo/services/notification/pkg/shared/domain"
)

// OtpRepository abstract interface
type OtpRepository interface {
	Save(ctx context.Context, data *shareddomain.OTPRequest) error
	FindLatestUnverified(ctx context.Context, recipient, purpose string) (shareddomain.OTPRequest, error)
	IncrementAttempt(ctx context.Context, id int) error
	MarkVerified(ctx context.Context, id int) error
}
