// Package tenantgorm 提供 GORM 多租户作用域和自动注入.
package tenantgorm

import (
	"context"

	"gorm.io/gorm"

	"github.com/Tsukikage7/servex/tenant"
)

const defaultColumn = "tenant_id"

// Scope 返回 GORM 查询作用域，自动按 tenant_id 过滤.
// 示例:
//
//	db.Scopes(tenantgorm.Scope(ctx)).Find(&results)
//	db.Scopes(tenantgorm.Scope(ctx, "t.tenant_id")).Find(&results)
func Scope(ctx context.Context, columns ...string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		id := tenant.ID(ctx)
		if id == "" {
			return db
		}
		col := defaultColumn
		if len(columns) > 0 && columns[0] != "" {
			col = columns[0]
		}
		return db.Where(col+" = ?", id)
	}
}

// AutoInject 注册 GORM 回调，在 Create/Update 时自动注入 tenant_id.
// 使用前需要确保模型包含对应的 tenant_id 字段.
// 示例:
//
//	if err := tenantgorm.AutoInject(db); err != nil {
//	    log.Fatal(err)
//	}
func AutoInject(db *gorm.DB, column ...string) error {
	col := defaultColumn
	if len(column) > 0 && column[0] != "" {
		col = column[0]
	}

	callback := func(db *gorm.DB) {
		if db.Statement.Context == nil {
			return
		}
		id := tenant.ID(db.Statement.Context)
		if id == "" {
			return
		}
		db.Statement.SetColumn(col, id)
	}

	if err := db.Callback().Create().Before("gorm:create").Register("tenant:auto_inject_create", callback); err != nil {
		return err
	}
	if err := db.Callback().Update().Before("gorm:update").Register("tenant:auto_inject_update", callback); err != nil {
		return err
	}
	return nil
}
