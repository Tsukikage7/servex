# middleware/signature

HMAC 请求签名验证中间件，用于 API 接口的请求合法性校验，防止重放攻击。

## 特性

- HMAC-SHA256 / HMAC-SHA512 签名
- 时间戳有效期检查（默认 5 分钟，防重放）
- HTTP 服务端验签中间件
- 客户端签名辅助函数（自动设置请求头）
- 常量时间比较，防止时序攻击

## 签名算法

```
HMAC-SHA256(secret, timestamp + "." + body)
```

- `X-Timestamp`：Unix 时间戳（秒）
- `X-Signature`：十六进制编码的 HMAC 摘要

## 服务端（中间件）

```go
import "github.com/Tsukikage7/servex/v2/middleware/signature"

// 使用默认配置（SHA256，MaxAge=5min）
cfg := signature.DefaultConfig("my-secret-key")
handler = signature.HTTPMiddleware(cfg)(handler)

// 自定义配置
cfg = &signature.Config{
    Secret:          "my-secret-key",
    HeaderName:      "X-Signature",   // 默认值
    TimestampHeader: "X-Timestamp",   // 默认值
    MaxAge:          10 * time.Minute,
    Algorithm:       "sha512",        // "sha256" 或 "sha512"
}
handler = signature.HTTPMiddleware(cfg)(handler)
```

## 客户端（签名请求）

```go
req, _ := http.NewRequest("POST", "https://api.example.com/data", body)

// 使用默认配置签名
_ = signature.SignRequest(req, "my-secret-key")

// 使用自定义配置签名
_ = signature.SignRequestWithConfig(req, cfg)

// 发送请求（headers 已自动设置）
resp, _ := http.DefaultClient.Do(req)
```

## 低级 API

```go
// 直接签名（不依赖 http.Request）
timestamp := strconv.FormatInt(time.Now().Unix(), 10)
sig := signature.Sign(body, timestamp, "my-secret")

// 验证签名
ok := signature.Verify(body, timestamp, sig, "my-secret")
```

## 错误响应

| 错误 | HTTP 状态码 | 说明 |
|---|---|---|
| `ErrMissingSignature` | 401 | 缺少签名头 |
| `ErrMissingTimestamp` | 401 | 缺少时间戳头 |
| `ErrExpiredTimestamp` | 401 | 时间戳超出有效期 |
| `ErrInvalidSignature` | 401 | 签名验证失败 |

## 与 httpserver 集成

```go
srv := httpserver.New(mux,
    httpserver.WithLogger(log),
    httpserver.WithMiddlewares(
        signature.HTTPMiddleware(signature.DefaultConfig("shared-secret")),
    ),
)
```
