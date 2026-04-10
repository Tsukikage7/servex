# Design: errors / httpclient / notification

> Date: 2026-04-04
> Status: Approved
> Modules: `errors/`, `httpclient/`, `notification/`

## 概述

为 servex 新增三个模块：统一错误码体系、HTTP 客户端封装、多渠道通知服务。均采用顶层包组织方式，与现有 pubsub/、jobqueue/、oauth2/ 风格一致。

实现顺序按依赖关系排列：errors → httpclient → notification。

---

## 1. errors/ — 统一错误码体系

### 设计目标

提供整数码 + 分层字符串码 + 消息的三合一错误定义，声明时绑定 HTTP 状态码和 gRPC Code 映射，消除各包各自定义错误的碎片化问题。

### 核心类型

```go
// Error 统一业务错误
type Error struct {
    Code     int               // 整数码，如 100401
    Key      string            // 分层字符串码，如 "auth.token.expired"
    Message  string            // 人类可读消息（默认中文）
    HTTP     int               // HTTP 状态码映射，如 401
    GRPC     codes.Code        // gRPC Code 映射，如 codes.Unauthenticated
    Metadata map[string]string // 可选附加信息
    cause    error             // 包装的底层错误
}
```

#### 方法

| 方法 | 说明 |
|---|---|
| `Error() string` | 实现 error 接口，格式：`[100401] auth.token.expired: 令牌已过期` |
| `Unwrap() error` | 支持 errors.Is / errors.As 链式解包 |
| `WithCause(err error) *Error` | 包装底层错误，返回新实例（不修改原定义） |
| `WithMeta(k, v string) *Error` | 附加元数据，返回新实例 |
| `WithMessage(msg string) *Error` | 覆盖消息（i18n 场景），返回新实例 |

**重要**：WithCause / WithMeta / WithMessage 均返回浅拷贝，不修改包级变量原始定义。

### 错误定义方式（Builder）

```go
var (
    ErrTokenExpired = errors.New(100401, "auth.token.expired", "令牌已过期").
        WithHTTP(401).WithGRPC(codes.Unauthenticated)

    ErrUserNotFound = errors.New(200404, "user.not_found", "用户不存在").
        WithHTTP(404).WithGRPC(codes.NotFound)

    ErrInternal = errors.New(900500, "internal", "服务内部错误").
        WithHTTP(500).WithGRPC(codes.Internal)
)
```

Builder 方法（仅用于定义阶段）：

| 方法 | 说明 |
|---|---|
| `New(code int, key, message string) *Error` | 创建错误定义 |
| `WithHTTP(status int) *Error` | 绑定 HTTP 状态码 |
| `WithGRPC(code codes.Code) *Error` | 绑定 gRPC Code |

### 辅助函数

```go
// 从 error 中提取 *Error
func FromError(err error) (*Error, bool)

// 判断是否匹配某个错误码（按 Code 字段比较，忽略 cause/metadata）
// 注意：不覆盖标准库 errors.Is，通过 Error 实现 Is(target error) bool 方法来兼容标准库
func CodeIs(err error, target *Error) bool

// HTTP/gRPC 响应转换
func ToHTTPStatus(err error) int              // 提取 HTTP 状态码，默认 500
func ToGRPCStatus(err error) *status.Status   // 转为 gRPC Status + details
func FromGRPCStatus(st *status.Status) *Error // 从 gRPC Status 还原
```

### 中间件

**HTTP 错误响应中间件**：拦截 handler 返回的错误，自动转为 JSON 响应：

```json
{
    "code": 100401,
    "key": "auth.token.expired",
    "message": "令牌已过期"
}
```

**gRPC 错误拦截器**：自动将 `*Error` 转为 gRPC Status，并通过 details 传递完整错误信息。

```go
func HTTPErrorHandler() func(http.Handler) http.Handler
func UnaryServerInterceptor() grpc.UnaryServerInterceptor
func StreamServerInterceptor() grpc.StreamServerInterceptor
```

### 文件结构

```
errors/
  errors.go        ← Error 类型、New、Is、FromError
  builder.go       ← builder 方法（WithHTTP、WithGRPC）
  http.go          ← HTTP 响应转换、中间件
  grpc.go          ← gRPC Status 转换、拦截器
  errors_test.go
```

---

## 2. httpclient/ — 统一 HTTP 客户端

### 设计目标

围绕 `net/http.Client` 做中间件链封装，复用现有 servex 组件（retry、circuitbreaker、tracing、metrics、discovery），不引入新外部依赖。

### 核心类型

```go
// Client 封装 http.Client，支持中间件链
type Client struct {
    http     *http.Client
    base     string             // 基础 URL
    mw       []Middleware        // RoundTripper 中间件链
    headers  http.Header         // 默认请求头
    logger   logger.Logger
}

// Middleware 作用于 RoundTripper 层
type Middleware func(http.RoundTripper) http.RoundTripper

// Request 请求描述
type Request struct {
    Method  string
    Path    string
    Body    any                  // 自动 JSON 序列化
    Headers map[string]string
    Query   map[string]string
}

// Response 响应包装
type Response struct {
    *http.Response
}
```

#### Response 便捷方法

| 方法 | 说明 |
|---|---|
| `JSON(v any) error` | 反序列化 JSON body 到目标 |
| `Text() (string, error)` | 读取文本 |
| `Bytes() ([]byte, error)` | 读取字节 |
| `CheckStatus() error` | 非 2xx 时返回 `*errors.Error` |

### 创建方式（Option Pattern）

```go
client := httpclient.New(
    httpclient.WithBaseURL("http://user-service:8080"),
    httpclient.WithTimeout(10 * time.Second),
    httpclient.WithLogger(log),
    httpclient.WithHeader("X-Service", "order-service"),
    httpclient.WithRetry(&retry.Config{MaxAttempts: 3, Delay: 100*time.Millisecond}),
    httpclient.WithCircuitBreaker(&circuitbreaker.Config{Threshold: 5}),
    httpclient.WithTracing("order-service"),
    httpclient.WithMetrics(collector),
    httpclient.WithDiscovery(disc, "user-service"),
)
```

### 请求 API

```go
// 便捷方法
resp, err := client.Get(ctx, "/api/v1/users/123")
resp, err := client.Post(ctx, "/api/v1/users", body)
resp, err := client.Put(ctx, "/api/v1/users/123", body)
resp, err := client.Delete(ctx, "/api/v1/users/123")

// 完整控制
resp, err := client.Do(ctx, &httpclient.Request{
    Method:  http.MethodPost,
    Path:    "/api/v1/users",
    Body:    createReq,
    Headers: map[string]string{"X-Request-ID": reqID},
    Query:   map[string]string{"page": "1"},
})
```

### 内置中间件（RoundTripper 层）

| 中间件 | 说明 | 复用来源 |
|---|---|---|
| retry | 重试（可重试状态码：502/503/504） | middleware/retry 的 Config 和退避策略 |
| circuitbreaker | 熔断 | middleware/circuitbreaker |
| tracing | 链路追踪传播（注入 traceparent 头） | observability/tracing |
| metrics | 请求计数/延迟/状态码分布 | observability/metrics |
| logging | 请求/响应日志 | 新写，使用 logger 接口 |
| discovery | 从服务发现解析地址替换 base URL | discovery |

中间件执行顺序（从外到内）：metrics → tracing → circuitbreaker → retry → logging → discovery → transport。

### Config 驱动（可选）

```go
type Config struct {
    BaseURL        string        `json:"base_url" yaml:"base_url" mapstructure:"base_url"`
    Timeout        time.Duration `json:"timeout" yaml:"timeout" mapstructure:"timeout"`
    MaxRetries     int           `json:"max_retries" yaml:"max_retries" mapstructure:"max_retries"`
    RetryDelay     time.Duration `json:"retry_delay" yaml:"retry_delay" mapstructure:"retry_delay"`
    CircuitBreaker bool          `json:"circuit_breaker" yaml:"circuit_breaker" mapstructure:"circuit_breaker"`
    Tracing        bool          `json:"tracing" yaml:"tracing" mapstructure:"tracing"`
}
```

### 文件结构

```
httpclient/
  client.go         ← Client 类型、New、Get/Post/Put/Delete/Do
  options.go        ← Option 定义
  response.go       ← Response 包装
  middleware.go     ← Middleware 类型、内置中间件适配器
  discovery.go      ← 服务发现 RoundTripper
  config.go         ← Config 结构体
  errors.go         ← 包级错误
  httpclient_test.go
```

---

## 3. notification/ — 多渠道通知服务

### 设计目标

统一接口抽象四个渠道（Email/SMS/Webhook/Push），内置模板引擎，支持异步发送（可选集成 jobqueue）。子包隔离模式与 pubsub/ 一致。

### 核心接口

```go
// Sender 统一发送接口
type Sender interface {
    Send(ctx context.Context, msg *Message) (*Result, error)
    Channel() Channel
    Close() error
}

// Channel 渠道枚举
type Channel string
const (
    ChannelEmail   Channel = "email"
    ChannelSMS     Channel = "sms"
    ChannelWebhook Channel = "webhook"
    ChannelPush    Channel = "push"
)

// Message 统一消息体
type Message struct {
    Channel      Channel           // 发送渠道
    To           []string          // 收件人
    Subject      string            // 主题（Email 用，其他可选）
    Body         string            // 正文（纯文本或渲染后 HTML）
    TemplateID   string            // 模板 ID（与 Body 二选一）
    TemplateData map[string]any    // 模板变量
    Metadata     map[string]string // 扩展字段
}

// Result 发送结果
type Result struct {
    MessageID string
    Channel   Channel
    Error     error
}

// TemplateEngine 模板渲染接口
type TemplateEngine interface {
    Render(templateID string, data map[string]any) (string, error)
}
```

### Dispatcher — 多渠道调度器

```go
type Dispatcher struct { ... }

func NewDispatcher(opts ...Option) *Dispatcher
func (d *Dispatcher) Register(sender Sender)
func (d *Dispatcher) Send(ctx context.Context, msg *Message) (*Result, error)
func (d *Dispatcher) Broadcast(ctx context.Context, channels []Channel, msg *Message) []*Result
func (d *Dispatcher) SendAsync(ctx context.Context, msg *Message) error // 需注入 jobqueue
```

Dispatcher Option：

| Option | 说明 |
|---|---|
| `WithLogger(log)` | 日志 |
| `WithTemplateEngine(tmpl)` | 模板引擎 |
| `WithJobQueue(client)` | 异步发送队列 |
| `WithDefaultChannel(ch)` | 默认渠道 |

### 渠道实现

#### Email (SMTP)

```go
sender, _ := email.NewSender(
    email.WithSMTP("smtp.example.com", 587),
    email.WithAuth("user@example.com", "password"),
    email.WithFrom("noreply@example.com", "My Service"),
    email.WithTLS(true),
)
```

Metadata 扩展键：`cc`, `bcc`, `reply_to`, `attachments`（逗号分隔路径）。

#### SMS

```go
// Provider 接口 — 各短信平台适配器实现此接口
type Provider interface {
    Send(ctx context.Context, req *SendRequest) (messageID string, err error)
    Name() string // 平台名称，如 "aliyun"、"tencent"
}

// SendRequest 短信发送请求
type SendRequest struct {
    Phone        string            // 手机号
    Content      string            // 内容（已渲染）
    SignName     string            // 短信签名（可选，来自 Metadata）
    TemplateCode string            // 平台模板码（可选，来自 Metadata）
    Params       map[string]string // 平台模板参数
}

sender, _ := sms.NewSender(
    sms.WithProvider(sms.NewAliyunProvider(&sms.AliyunConfig{
        AccessKeyID:     "xxx",
        AccessKeySecret: "xxx",
        SignName:        "我的服务",
    })),
)
```

内置 Provider：`AliyunProvider`、`TencentProvider`。

Metadata 扩展键：`sign_name`, `template_code`（短信平台模板码，区别于 notification 层模板）。

#### Webhook

```go
sender, _ := webhook.NewSender(
    webhook.WithTimeout(10 * time.Second),
    webhook.WithRetry(3),
)
```

To = webhook URL。Metadata 扩展键：`secret`（HMAC 签名）、`format`（`slack` / `dingtalk` / `lark` / `custom`）。

内置格式化器自动适配不同平台的 payload 结构。

#### Push (APNs + FCM)

```go
// Provider 接口
type Provider interface {
    Send(ctx context.Context, token string, payload *Payload) (messageID string, err error)
}

type Payload struct {
    Title string
    Body  string
    Data  map[string]string
    Badge int
    Sound string
}

sender, _ := push.NewSender(
    push.WithProvider(push.NewFCMProvider(&push.FCMConfig{CredentialsJSON: credJSON})),
    push.WithProvider(push.NewAPNsProvider(&push.APNsConfig{KeyFile: "key.p8", TeamID: "xxx", KeyID: "xxx"})),
)
```

To = device token。Push Sender 路由策略：优先使用 Metadata `platform` 键（值为 `fcm` 或 `apns`）；若未指定，则按 token 长度和格式启发式判断（64 位十六进制 = APNs，其余 = FCM）。如果只注册了一个 Provider 则直接使用。

### 模板引擎

内置基于 `html/template` 的实现，支持文件目录或 `embed.FS` 加载：

```go
tmpl := notification.NewTemplateEngine(
    notification.WithTemplateDir("templates/notifications"),
    // 或
    notification.WithTemplateFS(embeddedFS),
)
```

模板文件示例 `templates/notifications/welcome.html`：

```html
<h1>欢迎 {{.name}}</h1>
<p>感谢注册我们的服务。</p>
```

Dispatcher 在发送前，若 Message.TemplateID 非空，自动调用 TemplateEngine 渲染 Body。

### 异步发送

当 Dispatcher 注入了 jobqueue.Client 时，`SendAsync` 将消息序列化后投递到队列：

- Queue 名称：`notifications`
- Job Type：按渠道区分，如 `notification.email`、`notification.sms`
- 消费端注册对应 handler 调用 Sender.Send

### 文件结构

```
notification/
  notification.go      ← Channel, Message, Result, Sender 接口
  dispatcher.go        ← Dispatcher 调度器
  options.go           ← Dispatcher Option
  template.go          ← TemplateEngine 接口 + 内置实现
  errors.go            ← 包级错误
  email/
    sender.go          ← SMTP Sender
    options.go
  sms/
    sender.go          ← SMS Sender
    provider.go        ← Provider 接口
    aliyun.go          ← 阿里云 Provider
    tencent.go         ← 腾讯云 Provider
    options.go
  webhook/
    sender.go          ← Webhook Sender
    format.go          ← Slack/DingTalk/Lark 格式化器
    options.go
  push/
    sender.go          ← Push Sender
    provider.go        ← Provider 接口
    fcm.go             ← FCM Provider
    apns.go            ← APNs Provider
    options.go
  factory/
    factory.go         ← Config 驱动工厂
```

---

## 依赖关系

```
errors (无外部依赖，仅依赖 google.golang.org/grpc)
  ↑
httpclient (依赖 errors, middleware/retry, middleware/circuitbreaker,
            observability/tracing, observability/metrics, discovery, logger)
  ↑
notification (依赖 errors, logger; 可选依赖 jobqueue, httpclient)
```

## 实现顺序

1. **errors** — 基础设施，被其他模块依赖
2. **httpclient** — 独立模块，notification/webhook 和 notification/sms 可复用
3. **notification** — 最后实现，可使用前两个模块
