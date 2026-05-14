# middleware/cors

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/middleware/cors"
```

## 简介

`middleware/cors` 提供 HTTP CORS（跨源资源共享）中间件，处理预检请求（OPTIONS）和跨域请求头。默认允许所有来源，可通过选项限制允许的来源、方法、请求头，以及配置凭据和缓存时间。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `HTTPMiddleware(opts...)` | 返回 HTTP CORS 中间件 |
| `WithAllowOrigins(origins...)` | 设置允许的来源（默认 `["*"]`） |
| `WithAllowMethods(methods...)` | 设置允许的 HTTP 方法 |
| `WithAllowHeaders(headers...)` | 设置允许的请求头 |
| `WithExposeHeaders(headers...)` | 设置暴露给客户端的响应头 |
| `WithAllowCredentials(allow)` | 是否允许携带凭据 |
| `WithMaxAge(seconds)` | 预检结果缓存时间（默认 86400s） |

## 示例

```go
package main

import (
    "net/http"

    "github.com/Tsukikage7/servex/v2/middleware/cors"
)

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.Write([]byte(`{"users":[]}`))
    })

    // 允许特定来源
    corsHandler := cors.HTTPMiddleware(
        cors.WithAllowOrigins("https://app.example.com", "https://admin.example.com"),
        cors.WithAllowMethods("GET", "POST", "PUT", "DELETE"),
        cors.WithAllowHeaders("Content-Type", "Authorization", "X-Request-ID"),
        cors.WithExposeHeaders("X-Total-Count"),
        cors.WithAllowCredentials(true),
        cors.WithMaxAge(3600),
    )(mux)

    // 开发环境：允许所有来源（默认配置）
    devHandler := cors.HTTPMiddleware()(mux)
    _ = devHandler

    http.ListenAndServe(":8080", corsHandler)
}
```
