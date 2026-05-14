# storage/sqlx

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/storage/sqlx"
```

## 简介

`storage/sqlx` 提供泛型 `Nullable[T]` 类型，用于表示可为 NULL 的字段。实现了 `json.Marshaler/Unmarshaler`（JSON 中 null 与零值区分）和 `driver.Valuer/sql.Scanner`（数据库 NULL 读写），适用于 PATCH 请求和可选字段场景。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `Nullable[T]` | 可为 NULL 的泛型包装类型 |
| `Of[T](value)` | 创建有值的 Nullable |
| `Null[T]()` | 创建 NULL 值的 Nullable |
| `Nullable.ValueOr(defaultVal)` | 获取值，若为 NULL 返回默认值 |
| `Nullable.IsNull()` | 是否为 NULL |
| `Nullable.Get()` | 获取值和有效标志 |

## 示例

```go
package main

import (
    "encoding/json"
    "fmt"

    "github.com/Tsukikage7/servex/v2/storage/sqlx"
)

type UpdateUserRequest struct {
    Name  sqlx.Nullable[string]  `json:"name"`
    Age   sqlx.Nullable[int]     `json:"age"`
    Email sqlx.Nullable[string]  `json:"email"`
}

func main() {
    // JSON 反序列化：区分"未提供"和"显式置 null"
    body := `{"name":"Alice","age":null}`
    var req UpdateUserRequest
    json.Unmarshal([]byte(body), &req)

    fmt.Println("name 有值:", !req.Name.IsNull())   // true
    fmt.Println("age 为 null:", req.Age.IsNull())   // true
    fmt.Println("email 未提供:", req.Email.IsNull()) // true（零值也是 null）

    name, _ := req.Name.Get()
    fmt.Println("name 值:", name) // Alice

    // 创建 Nullable 值
    score := sqlx.Of(98.5)
    fmt.Println("score:", score.ValueOr(0.0)) // 98.5

    nullScore := sqlx.Null[float64]()
    fmt.Println("null score:", nullScore.ValueOr(-1.0)) // -1.0

    // JSON 序列化
    out, _ := json.Marshal(map[string]any{
        "score":      score,
        "null_score": nullScore,
    })
    fmt.Println(string(out)) // {"null_score":null,"score":98.5}
}
```
