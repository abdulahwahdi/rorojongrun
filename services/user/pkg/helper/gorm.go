package helper

import (
	"context"
	"time"

	"monorepo/globalshared"

	"github.com/golangid/candi/candishared"
	"gorm.io/gorm"
)

// DB returns the transaction stored in ctx by RepoSQL.WithTransaction, falling back to db,
// and attaches the tracing span. Reads inside a transaction use the transaction too.
// The result is a session: every statement chained from it starts from a clean copy, so the
// same value can be reused for several statements without conditions leaking between them.
func DB(ctx context.Context, db *gorm.DB) *gorm.DB {
	if tx, ok := candishared.GetValueFromContext(ctx, candishared.ContextKeySQLTransaction).(*gorm.DB); ok {
		db = tx
	}
	return globalshared.SetSpanToGorm(ctx, db).Session(&gorm.Session{})
}

// Alive scopes a query to rows that are not soft deleted
func Alive(db *gorm.DB, table string) *gorm.DB {
	return db.Where(table + ".deleted_at IS NULL")
}

// Save inserts model when id is 0, otherwise updates every column except id / created_at / deleted_at
// (zero values included, unlike gorm's default Updates)
func Save(db *gorm.DB, id int, model any) error {
	if id == 0 {
		return db.Create(model).Error
	}
	return db.Model(model).Select("*").Omit("id", "created_at", "deleted_at").Updates(model).Error
}

// SoftDelete marks the rows of model matching where as deleted
func SoftDelete(db *gorm.DB, model any, query string, args ...any) error {
	return db.Model(model).Where(query, args...).Where("deleted_at IS NULL").Update("deleted_at", time.Now()).Error
}
