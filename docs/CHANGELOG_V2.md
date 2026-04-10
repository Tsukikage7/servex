# servex v2.0.0 变更日志

发布日期: 2026-04-09

---

## 破坏性变更 (Breaking Changes)

### Logger 重构
- **移除** `Logger` 接口的 `WithContext(ctx context.Context) Logger` 方法
- **新增** `logger.NewContext(ctx, l)` 包级函数：将 Logger 存入 context
- **新增** `logger.FromContext(ctx)` 包级函数：从 context 取出 Logger（未找到时返回 nop logger）
- **新增** `logger.ContextWithTraceID(ctx, traceID)` / `logger.ContextWithSpanID(ctx, spanID)` 注入链路信息
- **新增** `logger.Nop()` 返回空日志实现
- **变更** 默认日志格式从 JSON 改为 Console，默认启用 caller（short 模式），字段样式改为 `[key:value]` 同行显示

### Server 中间件解耦
- **移除** `httpserver` 所有内置中间件 Option：`WithRecovery()`, `WithLogging()`, `WithTracing()`, `WithAuth()`, `WithMetrics()`, `WithRateLimit()`, `WithTenant()`, `WithClientIP()`
- **移除** `grpcserver` 所有内置中间件 Option：同上，以及 `WithPublicMethods()`, `WithAutoDiscovery()`, `WithTenantResolver()` 等
- **移除** `gateway` 所有内置中间件 Option
- **变更** Server 构造改为用户通过 `WithMiddlewares()` / `WithUnaryInterceptor()` / `WithStreamInterceptor()` 显式组合
- **新增** `httpserver`/`grpcserver` 自动将 logger 注入到每个请求的 context（最外层），无需手动添加
- **移除** `transport.GRPCConfig.PublicMethods` 字段

### 包删除
- **删除** `middleware/requestid` 包（功能由 `middleware/trace` 完全覆盖）

### middleware/trace 变更
- **移除** requestId 相关功能（`RequestIDHeader` 字段等）
- **变更** 只管 traceId + spanId 传播
- **新增** 优先复用 OTel span 的 traceId/spanId（OTel span > 请求头 > UUID 自动生成）
- **新增** `PropagateHeaders` 字段，支持自定义 header 传播
- **新增** `TraceIDFromContext(ctx)` 从 context 获取 trace ID
- **新增** `InjectHTTPHeaders(ctx, req)` 向下游 HTTP 请求注入 trace 信息
- **新增** `InjectGRPCMetadata(ctx)` 向下游 gRPC 调用注入 trace 信息

### GORM Store 拆分
- **移动** `auth/rbac.NewGORMStore` → `auth/rbac/gorm.NewGORMStore`
- **移动** `domain/outbox.NewGORMStore` → `domain/outbox/gorm.NewStore` / `NewStoreFromDB`
- **移动** `domain/eventsourcing.NewGORMEventStore` → `domain/eventsourcing/gorm.NewEventStore`
- **移动** `domain/eventsourcing.NewGORMSnapshotStore` → `domain/eventsourcing/gorm.NewSnapshotStore`
- **移动** `domain/saga` KV Store → `domain/saga/kvstore.NewStore`
- **移动** `bizx/audit` GORM Store → `bizx/audit/gorm`
- **移动** `bizx/retry` GORM Store → `bizx/retry/gorm`
- **移动** `llm/serving/apikey` GORM Store → `llm/serving/apikey/gorm`
- **移动** `llm/serving/billing` GORM Store → `llm/serving/billing/gorm`
- 主包保留接口定义和内存实现，GORM 实现独立子包，消除对 `gorm.io/gorm` 的强依赖

### 其他破坏性变更
- **统一** `Validatable` 接口到 `validation.Validatable`，原各包定义改为 type alias
- **移动** `httpx/activity` Kafka producer → `httpx/activity/kafka` 子包
- **移除** `httpx/activity` 对 `auth` 和 `messaging` 的直接依赖
- **变更** gRPC 错误处理从 JSON-in-message 改为标准 `errdetails`（向后兼容读取旧格式）

---

## 新特性 (Features)

### 安全
- **JWT 非对称签名**: 支持 RS256、ES256、EdDSA 签名方法，新增 `WithRSAKeys()`、`WithECDSAKeys()`、`WithEdDSAKeys()` 选项，PEM 密钥文件加载，向后兼容 HMAC

### 可观测性
- **OTel Metrics**: 新增 `OTelCollector` 实现 `Collector` 接口，支持 OTLP/Prometheus/stdout 导出，与 `PrometheusCollector` 相同的指标覆盖
- **slog 适配器**: `logger.NewSlogHandler(l)` 将 servex Logger 转为 `slog.Handler`；`logger.AsSlog(l)` 便捷转为 `*slog.Logger`；`logger.NewFromSlog(sl)` 将 `*slog.Logger` 转为 servex Logger
- **告警规则引擎**: `observability/alerting` 包，支持自定义告警规则
- **持续性能剖析**: `observability/profiling` 包，集成 `runtime/trace.FlightRecorder`
- **SLO/SLI 追踪**: `observability/slo` 包

### Go 现代特性
- **iter.Seq/Seq2 迭代器**: 12 个 collections 子包全部添加 `All()`/`Backward()` 等迭代器方法，有序集合支持正向+反向遍历，并发安全类型使用快照迭代
- **synctest 测试**: `ratelimit`/`circuitbreaker` 测试改用虚拟时钟，消除 `time.Sleep` 式 flaky test

### 错误处理
- **gRPC 标准错误详情**: `errors.ToGRPCStatus` 使用 `errdetails`（`ErrorInfo`/`BadRequest`/`RetryInfo`），`errors.FromGRPCStatus` 提取标准详情，向后兼容 JSON-in-message

### 开发体验
- **`servex dev` 热重载**: 基于 `fsnotify` 监听 `.go` 文件变更，500ms 防抖自动重启，支持 `--entry`/`--watch`/`--exclude` 选项
- **配置错误增强**: `ConfigFieldError` 带字段路径 + 来源 + 期望/实际值，`config.Load` 返回结构化错误
- **CLI 用 buf 替代 protoc**: 新增 `lint`/`breaking` 子命令

### 中间件
- **自适应限流降级**: `middleware/adaptive` 包
- **gzip 响应压缩**: `middleware/gzip` 包

### 存储
- **Neo4j 图数据库**: `storage/neo4j` 客户端
- **MinIO 对象存储**: `storage/minio` 客户端

### 配置
- **K8s ConfigMap/Secret 配置源**: `config/kubernetes`
- **Apollo 配置中心**: `config/apollo`
- **Nacos 配置源和服务发现**: `discovery/nacos`

### 业务工具
- **AB 测试**: `bizx/abtest`
- **工作流引擎**: `bizx/workflow`
- **进程内事件总线**: `messaging/eventbus`

### 其他
- **transport 统一 Registrar 接口**: HTTP/gRPC 统一注册模式
- **公共 skipper 抽取**: `transport.Skipper` 支持路径跳过

---

## Bug 修复 (Bug Fixes)

### Critical (10 项)
- **C-01**: 修复 RabbitMQ 并发发布确认错乱
- **C-02**: 修复 crypto 包验证码使用非安全随机数
- **C-03**: 修复可重入锁未基于 goroutine 身份
- **C-04**: 修复验证码比对时序攻击
- **C-05**: 修复微信 OAuth redirect_uri bug
- **C-06**: 修复 `memoryCache.Close()` 重复调用 panic
- **C-07**: 修复 `app.start()` 100ms 后错误丢弃（改为持续消费 errCh）
- **C-08**: 修复服务 ID 使用不安全随机数
- **C-09**: 修复 Gemini API Key 暴露在 URL
- **C-10**: 修复 LLM 护栏只检查 Content 不检查 Parts

### High (34 项)
- H-01 ~ H-34 全部修复（详见 CODE_REVIEW 文档）

### Medium (79 项)
- M-01 ~ M-79 全部修复

### Low (31 项)
- L-01 ~ L-31 全部修复

### 其他修复
- 修复 Outbox Relay 测试竞态条件
- 全面代码审查修复：安全/性能/可靠性/质量共 35 项

---

## 重构 (Refactoring)

- **Logger 架构**: `WithContext` → `NewContext`/`FromContext` 包级函数 + server 自动注入
- **Server 瘦身**: httpserver/grpcserver/gateway 移除全部内置中间件，改为用户显式组合
- **Store 拆分**: 8 个业务包 GORM Store 拆到独立 `gorm/` 子包，消除强依赖
- **Validatable 统一**: 5 处定义统一到 `validation.Validatable`
- **activity 解耦**: 移除 `auth`/`messaging` 直接依赖，Kafka producer 移到子包
- **架构解耦**: LLM 模块复用核心包
- **transport 统一**: Registrar 接口和中间件统一，抽取公共 skipper

---

## 文档 (Documentation)

- 添加 v2.0.0 路线图 (`docs/V2_ROADMAP.md`)
- Example 函数 100% 覆盖所有包（189 个文件）
- 补齐 transport 文档（Registrar/Router/WithMetrics/skipper）
- 规范化 Go 注释风格（全局）
- GoDoc 首页文档

---

## 测试 (Testing)

- synctest 虚拟时钟替代 `time.Sleep` 式并发测试
- 核心模块 Benchmark 基准测试
- CLI 全量测试
- GORM Store 独立测试（拆分后各子包自带测试）

---

## CLI 工具

- **`servex dev`**: 热重载开发服务器
- **`servex proto lint`/`breaking`**: 基于 buf 的 proto 检查
- **`servex add aggregate`**: 快捷添加 DDD 聚合
- **`servex add proto`**: 快捷添加 proto 文件
- **`servex gen client`/`entity`/`valueobject`**: 代码生成
- **`servex upgrade`**: CLI 自升级
- 强制 Wire 依赖注入并重命名 `ComponentDef`
- 交互式向导（Everforest Dark 主题）
- 六边形架构模板（port + adapter）

---

## 依赖

- Go >= 1.26.1
- buf 替代 protoc（proto 工具链）
- 新增 `google.golang.org/genproto/googleapis/rpc/errdetails`（gRPC 标准错误详情）
