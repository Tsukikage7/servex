# storage/clickhouse

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/storage/clickhouse"
```

## 简介

`storage/clickhouse` 提供 ClickHouse 客户端封装，基于 `clickhouse-go/v2` 实现。提供统一的 `Client` 接口用于执行查询、批量写入和 DDL 操作，支持通过配置结构体创建连接。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `Config` | ClickHouse 连接配置（DSN/Addr/Database/Username/Password） |
| `Client` | ClickHouse 客户端接口 |
| `NewClient(cfg)` | 根据配置创建客户端，返回错误 |
| `MustNewClient(cfg)` | 根据配置创建客户端，失败则 panic |

## 示例

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/Tsukikage7/servex/v2/storage/clickhouse"
)

type AccessLog struct {
    Timestamp time.Time `ch:"timestamp"`
    UserID    string    `ch:"user_id"`
    Action    string    `ch:"action"`
    Duration  int64     `ch:"duration_ms"`
}

func main() {
    cfg := clickhouse.Config{
        Addr:     []string{"localhost:9000"},
        Database: "analytics",
        Username: "default",
        Password: "",
    }

    client := clickhouse.MustNewClient(cfg)
    ctx := context.Background()

    // 批量写入访问日志
    rows := []AccessLog{
        {Timestamp: time.Now(), UserID: "u-001", Action: "login", Duration: 12},
        {Timestamp: time.Now(), UserID: "u-002", Action: "view_product", Duration: 45},
    }

    if err := client.BatchInsert(ctx, "INSERT INTO access_logs VALUES", rows); err != nil {
        panic(err)
    }

    // 查询统计
    var count uint64
    if err := client.QueryRow(ctx, "SELECT count() FROM access_logs WHERE action = ?", "login").Scan(&count); err != nil {
        panic(err)
    }
    fmt.Printf("登录次数: %d\n", count)
}
```
