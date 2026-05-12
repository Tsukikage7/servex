# errors

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/errors"
```

## 简介

`errors` 包提供统一的业务错误类型 `Error`，包含业务码（Code）、键（Key）、消息（Message）和业务语义（Kind）。HTTP 状态码与 gRPC Code 统一由 `Kind` 推导。支持错误链（`WithCause`）、元数据附加（`WithMeta`）和 `errors.Is` 按 Code 比较。内置 HTTP 响应写入和 gRPC Status 转换工具。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `Error` | 统一业务错误类型 |
| `New(code, key, message)` | 创建错误定义（通常作为包级变量） |
| `NewWithKind(code, key, message, kind)` | 创建带业务语义映射的错误定义 |
| `WithKind(kind)` | 绑定业务语义，HTTP/gRPC 映射由 Kind 推导 |
| `Kind.HTTPStatus()` | 获取 Kind 对应的 HTTP 状态码 |
| `Kind.GRPCCode()` | 获取 Kind 对应的 gRPC Code |
| `WithCause(err)` | 包装底层错误（返回新实例） |
| `WithMeta(key, value)` | 附加元数据（返回新实例） |
| `WithMessage(msg)` | 覆盖消息（返回新实例） |
| `FromError(err)` | 从 error 提取 `*Error` |
| `CodeIs(err, target)` | 按 Code 判断错误 |
| `WriteError(w, err)` | 将 `*Error` 写入 HTTP 响应（JSON） |
| `WriteErrorFrom(w, err)` | 将 error 写入 HTTP 响应 |
| `ToGRPCStatus(err)` | 转为 gRPC Status |

## 示例

```go
package main

import (
    "fmt"

    "github.com/Tsukikage7/servex/v2/errors"
)

// 定义错误（通常在包级别）
var (
    ErrUserNotFound = errors.NewWithKind(
        404001,
        "user_not_found",
        "用户不存在",
        errors.KindNotFound,
    )

    ErrUnauthorized = errors.NewWithKind(
        401001,
        "unauthorized",
        "未授权",
        errors.KindUnauthenticated,
    )
)

func main() {
    // 附加原始错误
    err := ErrUserNotFound.WithCause(fmt.Errorf("db: record not found"))
    fmt.Println(err) // [404001] user_not_found: 用户不存在: db: record not found

    // 附加元数据
    err2 := ErrUserNotFound.WithMeta("user_id", "u-123")

    // 按 Code 比较（errors.Is 兼容）
    fmt.Println(errors.CodeIs(err2, ErrUserNotFound)) // true

    // 从 error 中提取
    e, ok := errors.FromError(err2)
    if ok {
        fmt.Println("业务码:", e.Code)    // 404001
        fmt.Println("HTTP码:", e.HTTP)    // 404
        fmt.Println("元数据:", e.Metadata) // map[user_id:u-123]
    }

    // 写入 HTTP 响应
    mux := http.NewServeMux()
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        errors.WriteError(w, ErrUserNotFound)
    })
}
```
