package helper

import (
	"context"

	"monorepo/globalshared"

	"github.com/golangid/candi/candishared"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DB returns the transaction stored in ctx by RepoSQL.WithTransaction, falling back to db,
// and attaches the tracing span. Reads inside a transaction use the transaction too.
// The result is a session: every statement chained from it starts from a clean copy.
func DB(ctx context.Context, db *gorm.DB) *gorm.DB {
	if tx, ok := candishared.GetValueFromContext(ctx, candishared.ContextKeySQLTransaction).(*gorm.DB); ok {
		db = tx
	}
	return globalshared.SetSpanToGorm(ctx, db).Session(&gorm.Session{})
}

// InTransaction reports whether ctx carries a transaction started by RepoSQL.WithTransaction
func InTransaction(ctx context.Context) bool {
	_, ok := candishared.GetValueFromContext(ctx, candishared.ContextKeySQLTransaction).(*gorm.DB)
	return ok
}

// ForUpdate row-locks the selected rows until the surrounding transaction ends
func ForUpdate(db *gorm.DB) *gorm.DB {
	return db.Clauses(clause.Locking{Strength: "UPDATE"})
}

// ForUpdateSkipLocked row-locks the selected rows, skipping rows another transaction holds
func ForUpdateSkipLocked(db *gorm.DB) *gorm.DB {
	return db.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
}
