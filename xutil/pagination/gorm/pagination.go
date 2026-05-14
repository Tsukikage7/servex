// Package gorm provides GORM helpers for cursor pagination.
package gorm

import (
	"gorm.io/gorm"

	"github.com/Tsukikage7/servex/v2/xutil/pagination"
)

// Paginate 为 GORM 查询添加游标分页条件.
// orderField 是排序字段名，cursorValue 从 pagination.DecodeCursor 获得.
func Paginate(db *gorm.DB, req *pagination.CursorRequest, orderField string) *gorm.DB {
	req.Apply()

	query := db

	if req.Cursor != "" {
		values, err := pagination.DecodeCursor(req.Cursor)
		if err != nil || len(values) == 0 {
			return query.Where("1 = 0") // 无效游标返回空结果
		}
		cursorValue := values[0]
		if req.Direction == pagination.Backward {
			query = query.Where(orderField+" < ?", cursorValue)
		} else {
			query = query.Where(orderField+" > ?", cursorValue)
		}
	}

	if req.Direction == pagination.Backward {
		query = query.Order(orderField + " DESC")
	} else {
		query = query.Order(orderField + " ASC")
	}

	// 多取一条用于判断是否有更多数据.
	return query.Limit(req.Limit + 1)
}
