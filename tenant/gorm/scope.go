// Package tenantgorm 提供 GORM 多租户作用域和自动注入.
package tenantgorm

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/Tsukikage7/servex/tenant"
)

const defaultColumn = "tenant_id"

// Scope 返回 GORM 查询作用域，自动按 tenant_id 过滤.
//
// 注意: 若上下文中无租户信息则不添加过滤条件，查询将返回所有租户数据.
// 调用方应确保上下文中已设置租户 ID，或在无租户时主动处理.
//
// 示例:
//
//	db.Scopes(tenantgorm.Scope(ctx)).Find(&results)
//	db.Scopes(tenantgorm.Scope(ctx, "t.tenant_id")).Find(&results)
func Scope(ctx context.Context, columns ...string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		id := tenant.ID(ctx)
		if id == "" {
			// 无租户信息时添加不可能满足的条件，避免意外返回全局数据
			_ = db.AddError(fmt.Errorf("tenant: 上下文中未设置租户 ID，拒绝无租户查询"))
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
			// 无租户信息时记录警告，避免数据写入全局作用域
			_ = db.AddError(fmt.Errorf("tenant: AutoInject 未能从上下文获取租户 ID，拒绝无租户写入"))
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
