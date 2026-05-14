# ai/middleware

`ai/middleware` 包提供针对 `llm.ChatModel` 接口的中间件链，支持日志记录、指数退避重试、令牌桶限流和用量追踪。

## 功能特性

- **Chain**：将多个中间件组合为单一 `Middleware`
- **Logging**：记录模型名称、耗时、token 用量、错误信息
- **Retry**：对 429/5xx 错误自动指数退避重试
- **RateLimit**：对接 `middleware/ratelimit.Limiter` 限流
- **UsageTracker**：线程安全累计追踪 token 用量

## 安装

```bash
go get github.com/Tsukikage7/servex/v2/llm
```

## API

### Chain

```go
func Chain(outer Middleware, others ...Middleware) Middleware
```

第一个参数是最外层（最先执行），最后一个参数是最内层（最后执行）。

### Logging

```go
func Logging(log logger.Logger) Middleware
```

记录字段：`model`、`duration_ms`、`messages`、`finish_reason`、`prompt_tokens`、`completion_tokens`、`total_tokens`。

### Retry

```go
func Retry(maxAttempts int, baseDelay time.Duration) Middleware
```

- 对 `llm.IsRetryable(err)` 返回 `true` 的错误（429/5xx）重试
- 退避策略：`baseDelay * 2^attempt`，最大 30 秒
- 同时覆盖 `Generate` 和 `Stream`

### RateLimit

```go
func RateLimit(limiter ratelimit.Limiter) Middleware
```

复用 `middleware/ratelimit` 包的限流器，每次调用前执行 `limiter.Wait(ctx)`。

### UsageTracker

```go
type UsageTracker struct { ... }

func (t *UsageTracker) Middleware() Middleware
func (t *UsageTracker) Total() llm.Usage
func (t *UsageTracker) Reset()
```

## 使用示例

```go
import (
    aimw "github.com/Tsukikage7/servex/v2/llm/middleware"
    "github.com/Tsukikage7/servex/middleware/ratelimit"
)

// 构建中间件链：限流 → 重试 → 日志
limiter := ratelimit.NewTokenBucket(10, 1) // 10 req/s
tracker := &aimw.UsageTracker{}

chain := aimw.Chain(
    aimw.RateLimit(limiter),
    aimw.Retry(3, 500*time.Millisecond),
    aimw.Logging(log),
    tracker.Middleware(),
)

// 应用到 Provider
model := chain(openllm.New(apiKey))

// 查询累计用量
usage := tracker.Total()
fmt.Printf("总 tokens: %d\n", usage.TotalTokens)
```

## 许可证

详见项目根目录 LICENSE 文件。
