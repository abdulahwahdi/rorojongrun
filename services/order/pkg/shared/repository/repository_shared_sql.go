package repository

import (
	"context"
	"time"

	"monorepo/globalshared/gormx"
	shareddomain "monorepo/services/order/pkg/shared/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SequenceRepository hands out gapless numbers (order numbers, invoice numbers)
type SequenceRepository interface {
	// Next increments and returns the sequence of scope+period. The row stays locked until the
	// surrounding transaction ends, so a rolled back number is handed out again: no gaps.
	Next(ctx context.Context, scope, period string) (int64, error)
}

// OutboxRepository is the transactional outbox shared by every module that publishes events
type OutboxRepository interface {
	Save(ctx context.Context, data *shareddomain.Outbox) error
	// LockPending reads unpublished events with FOR UPDATE SKIP LOCKED; call inside WithTransaction
	LockPending(ctx context.Context, limit int) ([]shareddomain.Outbox, error)
}

type sequenceRepoSQL struct{ db *gorm.DB }

func (r *sequenceRepoSQL) Next(ctx context.Context, scope, period string) (n int64, err error) {
	seq := shareddomain.NumberSequence{Scope: scope, Period: period, LastNo: 1}
	err = gormx.DB(ctx, r.db).Clauses(
		clause.OnConflict{
			Columns:   []clause.Column{{Name: "scope"}, {Name: "period"}},
			DoUpdates: clause.Assignments(map[string]any{"last_no": gorm.Expr("number_sequences.last_no + 1")}),
		},
		clause.Returning{Columns: []clause.Column{{Name: "last_no"}}},
	).Create(&seq).Error
	return seq.LastNo, err
}

type outboxRepoSQL struct{ db *gorm.DB }

func (r *outboxRepoSQL) Save(ctx context.Context, data *shareddomain.Outbox) error {
	now := time.Now()
	data.UpdatedAt = now
	if data.ID == 0 {
		data.CreatedAt = now
		return gormx.DB(ctx, r.db).Create(data).Error
	}
	return gormx.DB(ctx, r.db).Model(data).Select("*").Omit("id", "created_at").Updates(data).Error
}

func (r *outboxRepoSQL) LockPending(ctx context.Context, limit int) (result []shareddomain.Outbox, err error) {
	err = gormx.ForUpdateSkipLocked(gormx.DB(ctx, r.db)).Where("published_at IS NULL").
		Order("id").Limit(limit).Find(&result).Error
	return
}
