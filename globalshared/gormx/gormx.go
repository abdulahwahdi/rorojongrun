// Package gormx holds the GORM helpers the SQL services of the monorepo share: the context
// transaction + tracing session, row locking and paging.
package gormx

import (
	"context"
	"strings"

	"monorepo/globalshared"

	"github.com/golangid/candi/candishared"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

// ApplyPaging adds ordering and limit/offset from a candi filter.
// orderBy is only honoured when it is in allowed, so clients cannot sort by arbitrary columns.
func ApplyPaging(db *gorm.DB, f *candishared.Filter, defaultOrder string, allowed ...string) *gorm.DB {
	order := defaultOrder
	for _, a := range allowed {
		if f.OrderBy == a {
			order = a
		}
	}
	db = db.Order(clause.OrderByColumn{Column: clause.Column{Name: order}, Desc: strings.ToUpper(f.Sort) != "ASC"})
	if f.Limit > 0 || !f.ShowAll {
		db = db.Limit(f.Limit).Offset(f.CalculateOffset())
	}
	return db
}

// Like builds an ILIKE pattern of a search term with wildcard characters escaped
func Like(term string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(term) + "%"
}
