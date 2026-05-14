# storage/migration

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/storage/migration"
```

## 简介

`storage/migration` 提供基于 Go DSL 的数据库迁移框架，使用 GORM 执行迁移事务。每条迁移由版本号、描述和 Up/Down 函数组成，通过 `Registry` 注册，`Runner` 执行升级或回滚操作，支持查询迁移状态。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `Migration` | 迁移定义（Version/Description/Up/Down） |
| `Registry` | 迁移注册表 |
| `NewRegistry()` | 创建注册表 |
| `Registry.Add(migrations...)` | 注册迁移 |
| `MigrationStatus` | 迁移状态（Version/Description/Applied/AppliedAt） |
| `Runner` | 迁移执行器接口 |
| `NewRunner(db, registry)` | 创建执行器 |
| `Runner.Up(ctx)` | 执行所有待执行迁移 |
| `Runner.UpTo(ctx, version)` | 执行到指定版本 |
| `Runner.Down(ctx)` | 回滚最后一条迁移 |
| `Runner.Status(ctx)` | 查询所有迁移状态 |

## 示例

```go
package main

import (
    "context"
    "fmt"

    "gorm.io/driver/postgres"
    "gorm.io/gorm"

    "github.com/Tsukikage7/servex/v2/storage/migration"
)

func main() {
    db, err := gorm.Open(postgres.Open("host=localhost user=postgres dbname=myapp"), &gorm.Config{})
    if err != nil {
        panic(err)
    }

    registry := migration.NewRegistry()
    registry.Add(
        migration.Migration{
            Version:     1,
            Description: "创建用户表",
            Up: func(tx *gorm.DB) error {
                return tx.Exec(`CREATE TABLE users (
                    id BIGSERIAL PRIMARY KEY,
                    email VARCHAR(255) UNIQUE NOT NULL,
                    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
                )`).Error
            },
            Down: func(tx *gorm.DB) error {
                return tx.Exec(`DROP TABLE IF EXISTS users`).Error
            },
        },
        migration.Migration{
            Version:     2,
            Description: "添加用户名字段",
            Up: func(tx *gorm.DB) error {
                return tx.Exec(`ALTER TABLE users ADD COLUMN name VARCHAR(100)`).Error
            },
            Down: func(tx *gorm.DB) error {
                return tx.Exec(`ALTER TABLE users DROP COLUMN name`).Error
            },
        },
    )

    runner := migration.NewRunner(db, registry)
    ctx := context.Background()

    if err := runner.Up(ctx); err != nil {
        panic(err)
    }

    statuses, _ := runner.Status(ctx)
    for _, s := range statuses {
        fmt.Printf("v%d %s: 已应用=%v\n", s.Version, s.Description, s.Applied)
    }
}
```
