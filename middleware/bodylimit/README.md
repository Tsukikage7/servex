# middleware/bodylimit

HTTP 请求体大小限制中间件，防止客户端发送过大请求体导致服务器资源耗尽。

超出限制时返回 `413 Request Entity Too Large`。

## 快速开始

```go
import "github.com/Tsukikage7/servex/v2/middleware/bodylimit"

// 限制为 1 MB（字节数直接指定）
mw := bodylimit.HTTPMiddleware(1 << 20)

srv := httpserver.New(mux,
    httpserver.WithMiddlewares(mw),
)
```

## 使用 ParseLimit 解析人类可读的大小

```go
// 支持 B, KB, MB, GB, TB（不区分大小写）
limit, err := bodylimit.ParseLimit("10MB")
if err != nil {
    log.Fatal(err)
}

mw := bodylimit.HTTPMiddleware(limit)
```

## 常用大小参考

```go
bodylimit.HTTPMiddleware(512 << 10)          // 512 KB（API 接口推荐）
bodylimit.HTTPMiddleware(1 << 20)            // 1 MB
bodylimit.HTTPMiddleware(10 << 20)           // 10 MB（文件上传）
bodylimit.HTTPMiddleware(100 << 20)          // 100 MB（大文件）

// 或使用 ParseLimit
limit, _ := bodylimit.ParseLimit("32MB")
bodylimit.HTTPMiddleware(limit)
```

## 实现机制

1. **Content-Length 快速检查**：若请求头声明的 `Content-Length` 已超限，立即返回 413，无需读取 body
2. **MaxBytesReader 包装**：用 `http.MaxBytesReader` 包装 `r.Body`，即便 Content-Length 缺失或伪造，实际传输也会被截断

## API

```go
// 创建请求体限制中间件，limit 为最大字节数
func HTTPMiddleware(limit int64) func(http.Handler) http.Handler

// 解析人类可读的大小字符串（如 "1MB", "512KB", "10.5GB"）
// 支持单位：B, KB, MB, GB, TB（不区分大小写）
func ParseLimit(s string) (int64, error)

// 预定义错误
var ErrBodyTooLarge = errors.New("bodylimit: request body too large")
```
