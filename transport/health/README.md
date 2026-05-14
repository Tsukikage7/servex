# health

`health` 包提供统一的健康检查功能，同时支持 HTTP 和 gRPC 协议，区分存活检查（Liveness）和就绪检查（Readiness）。

## 功能特性

- 区分存活检查和就绪检查，适配 Kubernetes 探针模型
- 内置数据库、Redis、Ping 等常用检查器
- 支持自定义检查器和函数式检查器
- 并发执行所有检查器，支持超时控制
- HTTP 处理器：自动注册 `/healthz` 和 `/readyz` 端点
- HTTP 中间件：无侵入式拦截健康检查路径
- gRPC 健康检查：实现 `grpc.health.v1.Health` 标准服务
- 支持手动覆盖服务状态

## 安装

```bash
go get github.com/Tsukikage7/servex/v2/transport/health
```

## API

### Health 管理器

```go
func New(opts ...Option) *Health
func (h *Health) AddLivenessChecker(checkers ...Checker)
func (h *Health) AddReadinessChecker(checkers ...Checker)
func (h *Health) Liveness(ctx context.Context) Response
func (h *Health) Readiness(ctx context.Context) Response
func (h *Health) IsHealthy(ctx context.Context) bool
```

### 配置选项

| 选项                    | 默认值 | 说明           |
| ----------------------- | ------ | -------------- |
| `WithTimeout`           | `5s`   | 检查超时时间   |
| `WithLivenessChecker`   | -      | 添加存活检查器 |
| `WithReadinessChecker`  | -      | 添加就绪检查器 |

### Checker 接口

```go
type Checker interface {
    Name() string
    Check(ctx context.Context) CheckResult
}
```

### CheckerFunc

函数式检查器适配器：

```go
checker := health.NewCheckerFunc("custom", func(ctx context.Context) health.CheckResult {
    if err := someCheck(); err != nil {
        return health.CheckResult{Status: health.StatusDown, Message: err.Error()}
    }
    return health.CheckResult{Status: health.StatusUp}
})
```

### 内置检查器

| 检查器                 | 说明                                    |
| ---------------------- | --------------------------------------- |
| `NewDBChecker`         | 数据库检查器，接受 Pinger 接口          |
| `NewDBCheckerFromSQL`  | 从 `*sql.DB` 创建数据库检查器           |
| `NewRedisChecker`      | Redis 检查器，接受 Pinger 接口          |
| `NewPingChecker`       | 通用 Ping 检查器                        |
| `NewCompositeChecker`  | 组合多个检查器为一个                    |
| `NewAlwaysUpChecker`   | 始终返回 UP 的检查器                    |

### 状态类型

| 常量            | 值        | 说明       |
| --------------- | --------- | ---------- |
| `StatusUp`      | `UP`      | 服务健康   |
| `StatusDown`    | `DOWN`    | 服务不健康 |
| `StatusUnknown` | `UNKNOWN` | 状态未知   |

### HTTP 处理器

```go
handler := health.NewHTTPHandler(h)
handler.RegisterRoutes(mux)              // 注册 /healthz 和 /readyz
handler.LivenessHandler()                // 返回存活检查 HandlerFunc
handler.ReadinessHandler()               // 返回就绪检查 HandlerFunc

// 便捷函数
health.LivenessHandlerFunc(h)
health.ReadinessHandlerFunc(h)

// 中间件方式（自动拦截 /healthz 和 /readyz）
wrapped := health.Middleware(h)(yourHandler)
```

### gRPC 健康检查服务

实现 `grpc.health.v1.Health` 标准接口：

```go
grpcServer := health.NewGRPCServer(h)
grpcServer.Register(server)                      // 注册到 gRPC 服务器
grpcServer.Check(ctx, req)                        // 执行健康检查
grpcServer.Watch(req, stream)                     // 流式健康监控
grpcServer.SetServingStatus("svc", status)        // 手动覆盖状态
```

### 响应结构

```go
type Response struct {
    Status    Status                 // 整体状态
    Timestamp time.Time              // 检查时间
    Duration  time.Duration          // 耗时
    Checks    map[string]CheckResult // 各检查器结果
}

type CheckResult struct {
    Status   Status         // 检查状态
    Message  string         // 错误信息
    Duration time.Duration  // 检查耗时
    Details  map[string]any // 附加详情
}
```

## 许可证

详见项目根目录 LICENSE 文件。
