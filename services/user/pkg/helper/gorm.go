package helper

import (
	"time"

	"gorm.io/gorm"
)

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
