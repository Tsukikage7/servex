// Package gorm provides GORM helpers for sorting.
package gorm

import (
	"gorm.io/gorm"

	"github.com/Tsukikage7/servex/v2/xutil/sorting"
)

// Scope 返回 GORM scope 函数，用于链式调用.
//
// 使用示例:
//
//	db.Scopes(sortgorm.Scope(sorting.New("created_at:desc"))).Find(&users)
func Scope(s sorting.Sorting) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if s.IsEmpty() {
			return db
		}
		return db.Order(s.String())
	}
}

// Apply 应用排序到 GORM 查询.
//
// 使用示例:
//
//	sortgorm.Apply(db, sorting.New("created_at:desc")).Find(&users)
func Apply(db *gorm.DB, s sorting.Sorting) *gorm.DB {
	if s.IsEmpty() {
		return db
	}
	return db.Order(s.String())
}
