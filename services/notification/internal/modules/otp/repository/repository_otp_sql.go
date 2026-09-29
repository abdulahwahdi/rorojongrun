package repository

import (
	"context"
	"time"

	shareddomain "monorepo/services/notification/pkg/shared/domain"

	"github.com/golangid/candi/tracer"

	"monorepo/globalshared"

	"gorm.io/gorm"
)

type otpRepoSQL struct {
	readDB, writeDB *gorm.DB
}

// NewOtpRepoSQL sql repo constructor
func NewOtpRepoSQL(readDB, writeDB *gorm.DB) OtpRepository {
	return &otpRepoSQL{readDB: readDB, writeDB: writeDB}
}

func (r *otpRepoSQL) Save(ctx context.Context, data *shareddomain.OTPRequest) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "OtpRepoSQL:Save")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	data.CreatedAt = time.Now()
	data.UpdatedAt = time.Now()
	err = globalshared.SetSpanToGorm(ctx, r.writeDB).Create(data).Error
	return
}

func (r *otpRepoSQL) FindLatestUnverified(ctx context.Context, recipient, purpose string) (result shareddomain.OTPRequest, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "OtpRepoSQL:FindLatestUnverified")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = globalshared.SetSpanToGorm(ctx, r.readDB).
		Where("recipient = ? AND purpose = ? AND verified_at IS NULL", recipient, purpose).
		Order("created_at DESC").
		First(&result).Error
	return
}

func (r *otpRepoSQL) IncrementAttempt(ctx context.Context, id int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "OtpRepoSQL:IncrementAttempt")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = globalshared.SetSpanToGorm(ctx, r.writeDB).Model(&shareddomain.OTPRequest{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"attempt_count": gorm.Expr("attempt_count + 1"),
			"updated_at":    time.Now(),
		}).Error
	return
}

func (r *otpRepoSQL) MarkVerified(ctx context.Context, id int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "OtpRepoSQL:MarkVerified")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	now := time.Now()
	err = globalshared.SetSpanToGorm(ctx, r.writeDB).Model(&shareddomain.OTPRequest{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"verified_at": now,
			"updated_at":  now,
		}).Error
	return
}
