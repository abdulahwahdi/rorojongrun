package repository

import (
	"context"
	"time"

	"monorepo/services/payment/pkg/helper"
	shareddomain "monorepo/services/payment/pkg/shared/domain"

	"github.com/golangid/candi/tracer"
	"gorm.io/gorm"
)

type gatewayRepoSQL struct {
	readDB, writeDB *gorm.DB
}

// NewGatewayRepoSQL repo constructor. Reads use the write connection: gateway config is
// read-after-written by the admin API and must not lag behind a replica.
func NewGatewayRepoSQL(readDB, writeDB *gorm.DB) GatewayRepository {
	return &gatewayRepoSQL{readDB: readDB, writeDB: writeDB}
}

func (r *gatewayRepoSQL) FetchAll(ctx context.Context) (data []shareddomain.Gateway, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "GatewayRepoSQL:FetchAll")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = helper.DB(ctx, r.writeDB).Order("id ASC").Find(&data).Error
	return
}

func (r *gatewayRepoSQL) FindByCode(ctx context.Context, code string) (result shareddomain.Gateway, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "GatewayRepoSQL:FindByCode")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = helper.DB(ctx, r.writeDB).Where("code = ?", code).First(&result).Error
	return
}

func (r *gatewayRepoSQL) Save(ctx context.Context, data *shareddomain.Gateway) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "GatewayRepoSQL:Save")
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
