package helper

import (
	"strings"

	"github.com/golangid/candi/candishared"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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

// StrPtr returns nil for an empty string, so optional unique columns store NULL instead of ”
func StrPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// StrVal dereferences a possibly nil string
func StrVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
