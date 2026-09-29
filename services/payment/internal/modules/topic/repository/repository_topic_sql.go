package repository

import (
	"context"
	"time"

	"monorepo/services/payment/internal/modules/topic/domain"
	"monorepo/services/payment/pkg/helper"
	shareddomain "monorepo/services/payment/pkg/shared/domain"

	"github.com/golangid/candi/tracer"
	"gorm.io/gorm"
)

type topicRepoSQL struct {
	readDB, writeDB *gorm.DB
}

// NewTopicRepoSQL repo constructor. Reads use the write connection so topic changes are
// visible to the consumer reload immediately.
func NewTopicRepoSQL(readDB, writeDB *gorm.DB) TopicRepository {
	return &topicRepoSQL{readDB: readDB, writeDB: writeDB}
}

func (r *topicRepoSQL) filter(db *gorm.DB, f *domain.FilterTopic) *gorm.DB {
	if f.Direction != "" {
		db = db.Where("direction = ?", f.Direction)
	}
	if f.GatewayCode != "" {
		db = db.Where("gateway_code = ?", f.GatewayCode)
	}
	if f.IsEnabled != nil {
		db = db.Where("is_enabled = ?", *f.IsEnabled)
	}
	if f.Search != "" {
		db = db.Where("topic ILIKE ?", helper.Like(f.Search))
	}
	return db
}

func (r *topicRepoSQL) FetchAll(ctx context.Context, f *domain.FilterTopic) (data []shareddomain.Topic, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "TopicRepoSQL:FetchAll")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	db := r.filter(helper.DB(ctx, r.writeDB), f)
	err = helper.ApplyPaging(db, &f.Filter, "id", "topic", "direction", "created_at", "updated_at").Find(&data).Error
	return
}

func (r *topicRepoSQL) Count(ctx context.Context, f *domain.FilterTopic) int {
	trace, ctx := tracer.StartTraceWithContext(ctx, "TopicRepoSQL:Count")
	defer trace.Finish()

	var total int64
	r.filter(helper.DB(ctx, r.writeDB), f).Model(&shareddomain.Topic{}).Count(&total)
	return int(total)
}

func (r *topicRepoSQL) FindByID(ctx context.Context, id int) (result shareddomain.Topic, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "TopicRepoSQL:FindByID")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = helper.DB(ctx, r.writeDB).Where("id = ?", id).First(&result).Error
	return
}

func (r *topicRepoSQL) Save(ctx context.Context, data *shareddomain.Topic) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "TopicRepoSQL:Save")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	now := time.Now()
	data.UpdatedAt = now
	db := helper.DB(ctx, r.writeDB)
	if data.ID == 0 {
		data.CreatedAt = now
		return db.Create(data).Error
	}
	return db.Model(data).Select("*").Omit("id", "created_at").Updates(data).Error
}

func (r *topicRepoSQL) Delete(ctx context.Context, id int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "TopicRepoSQL:Delete")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	return helper.DB(ctx, r.writeDB).Where("id = ?", id).Delete(&shareddomain.Topic{}).Error
}

func (r *topicRepoSQL) FetchEnabledConsume(ctx context.Context) (data []shareddomain.Topic, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "TopicRepoSQL:FetchEnabledConsume")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = helper.DB(ctx, r.writeDB).
		Table("payment_kafka_topics t").
		Select("t.*").
		Joins("JOIN payment_gateways g ON g.code = t.gateway_code AND g.is_enabled = true").
		Where("t.direction = ? AND t.is_enabled = true", shareddomain.TopicConsume).
		Order("t.id ASC").
		Find(&data).Error
	return
}

func (r *topicRepoSQL) FetchEnabledPublish(ctx context.Context, eventType string) (data []shareddomain.Topic, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "TopicRepoSQL:FetchEnabledPublish")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = helper.DB(ctx, r.writeDB).
		Where("direction = ? AND event_type = ? AND is_enabled = true", shareddomain.TopicPublish, eventType).
		Order("id ASC").Find(&data).Error
	return
}

func (r *topicRepoSQL) FindConsumeByTopic(ctx context.Context, topic string) (result shareddomain.Topic, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "TopicRepoSQL:FindConsumeByTopic")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = helper.DB(ctx, r.writeDB).Where("direction = ? AND topic = ?", shareddomain.TopicConsume, topic).First(&result).Error
	return
}
