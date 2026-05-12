# middleware/logging

## 导入路径

```go
import "github.com/Tsukikage7/servex/middleware/logging"
```

## 简介

`middleware/logging` 提供 HTTP 请求和 gRPC 调用的日志记录中间件。每次请求结束后记录方法、路径、状态码、耗时和响应字节数。支持跳过特定路径（如健康检查、metrics 端点）。

## 核心函数

| 函数 | 说明 |
|---|---|
| `HTTPMiddleware(opts...)` | HTTP 请求日志中间件 |
| `UnaryServerInterceptor(opts...)` | gRPC 一元服务器拦截器 |
| `StreamServerInterceptor(opts...)` | gRPC 流式服务器拦截器 |
| `WithLogger(l)` | 设置日志记录器（必需） |
| `WithSkipPaths(paths...)` | 设置跳过的路径 |

## 示例

```go
package main

import (
    "net/http"

    "google.golang.org/grpc"

    "github.com/Tsukikage7/servex/middleware/logging"
    "github.com/Tsukikage7/servex/v2/observability/logger"
)

func main() {
    log := logger.NewZap()

    // HTTP 日志中间件
    mux := http.NewServeMux()
    mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    })

    handler := logging.HTTPMiddleware(
        logging.WithLogger(log),
        logging.WithSkipPaths("/health", "/metrics", "/readyz"),
    )(mux)

    // 启动 HTTP 服务
    go http.ListenAndServe(":8080", handler)

    // gRPC 服务器日志中间件
    srv := grpc.NewServer(
        grpc.ChainUnaryInterceptor(
            logging.UnaryServerInterceptor(
                logging.WithLogger(log),
                logging.WithSkipPaths("/grpc.health.v1.Health/Check"),
            ),
        ),
        grpc.ChainStreamInterceptor(
            logging.StreamServerInterceptor(
                logging.WithLogger(log),
            ),
        ),
    )
    _ = srv
    // 日志格式示例：
    // INFO [http] method=GET path=/api/users status=200 duration=1.2ms bytes=42
    // INFO [grpc] method=/UserService/GetUser code=OK duration=5ms
}
```
