# tenant/gorm

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/tenant/gorm"
```

## 简介

`tenant/gorm` 提供 GORM 多租户作用域工具，自动为查询注入租户 ID 过滤条件，以及为创建/更新操作自动写入租户 ID 字段，实现数据库层面的租户隔离。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `Scope(ctx, columns...)` | 返回 GORM scope 函数，自动添加租户 ID WHERE 条件 |
| `AutoInject(db, column...)` | 返回注入了租户 scope 的 `*gorm.DB` |

## 示例

```go
package main

import (
    "context"
    "fmt"

    "gorm.io/driver/postgres"
    "gorm.io/gorm"

    "github.com/Tsukikage7/servex/v2/tenant"
    tenantgorm "github.com/Tsukikage7/servex/v2/tenant/gorm"
)

type Order struct {
    ID       uint   `gorm:"primaryKey"`
    TenantID string `gorm:"column:tenant_id"`
    Amount   float64
    Status   string
}

// SimpleTenant 简单租户
type SimpleTenant struct{ id string }

func (t *SimpleTenant) ID() string { return t.id }

func main() {
    db, err := gorm.Open(postgres.Open("host=localhost user=postgres dbname=myapp"), &gorm.Config{})
    if err != nil {
        panic(err)
    }

    // 模拟租户 context
    ctx := tenant.WithTenant(context.Background(), &SimpleTenant{id: "tenant-a"})

    // 查询时自动过滤当前租户的数据
    var orders []Order
    db.WithContext(ctx).
        Scopes(tenantgorm.Scope(ctx, "tenant_id")).
        Find(&orders)
    fmt.Printf("租户 %s 的订单数: %d\n", tenant.ID(ctx), len(orders))

    // 创建时自动注入租户 ID
    newOrder := Order{Amount: 199.00, Status: "pending"}
    tenantgorm.AutoInject(db.WithContext(ctx), "tenant_id").Create(&newOrder)
    fmt.Println("创建成功，租户 ID:", newOrder.TenantID)
}
```
