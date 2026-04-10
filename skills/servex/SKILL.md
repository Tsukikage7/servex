---
name: servex
description: servex Go 微服务工具库专家。当用户在使用 servex（项目依赖 github.com/Tsukikage7/servex）时触发，提供模块索引、代码生成规范和工作流程。包含 testx 测试工具包（NopLogger/TestLogger/Container/HTTPTest/Fixture）。
---

# servex 使用指南

## 触发条件

**触发：**
- 用户描述业务意图，如"我需要一个带熔断的 HTTP 服务"
- 用户明确说出 servex 模块名，如"帮我用 servex 的 circuitbreaker"
- 用户在 servex 项目中问某功能怎么实现

**不触发：**
- 问题涉及其他库（go-zero、kratos、gin 等），servex 只是对比提及
- 纯粹的 Go 语言问题，与 servex API 无关
- 无法确认用户项目使用 servex 时，先询问确认，而非直接生成

## 工作流程

1. **业务意图（模糊描述）** → 映射到合适的 servex 模块（1-2 句说明理由）→ 生成完整可运行示例
2. **模块名明确** → 直接生成该模块的示例或片段
3. **需求模糊，无法映射** → 先问一个澄清问题，再生成
4. **不确定 API 细节**（函数签名、选项名、默认值）→ 读取 servex 源码对应文件，不猜测
5. **生成多中间件代码** → 严格按此顺序：waf → logging → tracing → metrics → ratelimit → circuitbreaker → retry → timeout → recovery

**源码定位规则：**
- 若在 servex 仓库本身，源码根目录即为当前目录
- 否则在 go.mod 的 `replace` 指令或 module cache 中找到源码
- 按包路径直接定位目录，如不确定 `middleware/circuitbreaker/` 的选项，读取该目录下的 `.go` 文件

## 子 Skill 参考文档

当需要某个模块的详细 API 和示例时，使用 Read 工具读取对应的子 skill 文件：

| 领域 | 文件路径 | 覆盖模块 |
|------|---------|---------|
| 传输层 | `skills/servex/references/transport.md` | httpserver/grpcserver/httpclient/grpcclient/gateway/graphql/websocket/sse/grpcx/tls/debug |
| 中间件 | `skills/servex/references/middleware.md` | ratelimit/circuitbreaker/retry/recovery/timeout/cors/idempotency/semaphore/logging/secure/csrf/bodylimit/signature/trace/gzip/adaptive/waf/version/fallback/loadshed |
| 存储 | `skills/servex/references/storage.md` | cache/rdbms/mongodb/elasticsearch/clickhouse/s3/minio/neo4j/lock/sqlx/migration/redis |
| 认证 | `skills/servex/references/auth.md` | jwt/apikey/rbac |
| 可观测性 | `skills/servex/references/observability.md` | logger/metrics/tracing/logshipper/slo/alerting/profiling |
| 配置 | `skills/servex/references/config.md` | config/source/file/etcd/consul/env/nacos/apollo/k8s/discovery |
| 消息 | `skills/servex/references/pubsub.md` | pubsub/jobqueue (kafka/rabbitmq/redis)/eventbus |
| 领域驱动 | `skills/servex/references/distributed.md` | cqrs/saga/outbox/eventsourcing |
| 业务组件 | `skills/servex/references/bizx.md` | counter/leaderboard/sequence/locking/ratelimit/statemachine/pagination/audit/feature/retry/event/captcha/workflow/abtesting |
| 通知 | `skills/servex/references/notify.md` | email/sms/push/webhook |
| OAuth2 | `skills/servex/references/oauth2.md` | github/google/wechat/state |
| 其他 | `skills/servex/references/errors.md` `skills/servex/references/httpx.md` `skills/servex/references/i18n.md` `skills/servex/references/tenant.md` `skills/servex/references/validation.md` `skills/servex/references/xutil.md` `skills/servex/references/collections.md` `skills/servex/references/testx.md` `skills/servex/references/openapi.md` `skills/servex/references/webhook.md` | 对应模块 |

**使用方式：** 先在本 skill 的模块索引中定位目标模块，如需详细 API 和代码示例，Read 对应的子 skill 文件。

## 代码生成规范

- **完整示例**（`main.go` 级别）：用户需求模糊/业务层面时生成
- **片段**（函数/结构体级别）：用户需求具体/技术层面时生成
- 注释中文，选项模式（`WithXxx`），错误处理显式，不 panic
- 输出格式：先说明选择的模块及原因（1-2 句），再给出代码，最后附关键配置项说明

## 模块索引

### CLI 工具

| 模块 | 包路径 | 描述 | 核心类型/函数 |
|------|--------|------|--------------|
| cmd/servex | `cmd/servex` | 脚手架 CLI（交互式向导 [charmbracelet/huh, Everforest Dark 主题]、项目生成、DDD 代码生成、Proto 管理[基于 buf]） | `servex new`, `servex add service`, `servex gen aggregate/entity/valueobject/client/dockerfile/justfile`, `servex proto add/client/server/lint/breaking`, `servex run`, `servex dev`, `servex gen k8s`, `servex upgrade`, `servex completion` |

**CLI 详情：**

- **交互式向导：** `servex new` / `servex add service` 无参数时自动启动向导（Everforest Dark 主题）
- **new：** 创建 monorepo[默认]或 `--standalone` 单服务项目，支持 `--with-grpc`、`--infra`（Wire DI 强制启用）
- **add service：** 在 monorepo 中添加微服务，支持 `--with-grpc`、`--with-gateway`、`--infra`、`--observe`、`--auth`、`--discovery`、`--other`
- **add aggregate：** 生成 DDD 聚合[domain + application + adapter]，支持 `--commands`、`--unique`、`--service`
- **add proto：** 添加 proto 服务定义并生成服务端桩代码，支持 `--service`
- **gen aggregate：** 同 `add aggregate`，兼容旧用法
- **gen entity：** 生成 DDD 子实体（有 ID，可变，属于聚合），需 `--aggregate`
- **gen valueobject：** 生成 DDD 值对象（无 ID，不可变，属于聚合），需 `--aggregate`
- **gen client：** 生成外部服务客户端适配器（防腐层 + gRPC client），需 `--service`
- **gen dockerfile / justfile：** 生成 Dockerfile / justfile
- **proto add/client/server/lint/breaking：** Proto 模板创建（自动生成 buf.yaml + buf.gen.yaml）、buf generate 客户端代码生成、服务端桩代码生成、buf lint 规范检查、buf breaking 兼容性检测
- **run：** 运行服务（自动检测入口）
- **dev：** 开发模式，监听文件变更自动重启服务
- **gen k8s：** 生成 K8s Deployment + Service manifest（`--name`、`--port`、`--replicas`、`--image`）
- **upgrade / completion：** 自升级、Shell 自动补全（bash/zsh/fish）

**可用基础设施 Flag：**

| Flag | 可选值 |
|------|--------|
| --infra | mysql, postgres, sqlite, redis, mongo, es, clickhouse, s3, minio, neo4j, kafka, rabbitmq |
| --observe | metrics, tracing, profiling |
| --auth | jwt, rbac |
| --discovery | consul, etcd, nacos |
| --other | scheduler, i18n, tenant |

**生成的项目结构：** monorepo + 六边形架构（domain/ + application/ + services/ + api/）

### 传输层 → 详见 `transport` skill

| 模块 | 包路径 | 描述 | 核心类型/函数 |
|------|--------|------|--------------|
| httpserver | `transport/httpserver` | HTTP 服务器（集成中间件链） | `New`, `WithAddr`, `WithLogger`, `WithAuth`, `WithMiddlewares` |
| grpcserver | `transport/grpcserver` | gRPC 服务器 | `New`, `Server` |
| httpclient | `transport/httpclient` | HTTP 客户端（负载均衡） | `New`, `WithServiceName`, `WithDiscovery`, `WithBalancer` |
| ginserver | `transport/ginserver` | Gin 适配器 | `New` |
| echoserver | `transport/echoserver` | Echo 适配器 | `New` |
| hertzserver | `transport/hertzserver` | Hertz 适配器 | `New` |
| websocket | `transport/websocket` | WebSocket 服务端 | `NewServer`, `Handler` |
| sse | `transport/sse` | Server-Sent Events 服务端 | `NewServer`, `Handler` |
| gateway | `transport/gateway` | gRPC + HTTP 双协议服务器 | `New`, `Register`, `Registrar`, `WithAuth`, `WithPublicMethods` |
| grpcclient | `transport/grpcclient` | gRPC 客户端 | `New`, `Conn`, `WithServiceName`, `WithDiscovery` |
| health | `transport/health` | 健康检查 | `New`, `Checker`, `NewDBChecker`, `NewRedisChecker`, `Middleware` |
| response | `transport/response` | 统一响应格式 | `OK`, `Fail`, `Response`, `Code`, `BusinessError`, `ExtractCode` |
| graphql | `transport/graphql` | GraphQL 服务器适配 | `New`, `Handler`, `PlaygroundHandler`, `LoggingMiddleware`, `TracingMiddleware`, `RecoveryMiddleware`, `WrapResolve` |
| tls | `transport/tls` | TLS 配置工具（证书/mTLS/版本控制） | `NewServerTLSConfig`, `NewClientTLSConfig`, `NewTLSConfig` |
| grpcx | `transport/grpcx` | gRPC 工具包（流包装/Metadata/错误/健康检查） | `WrapServerStream`, `GetMetadataValue`, `AppendOutgoingMetadata`, `CopyIncomingToOutgoing`, `NotFound`, `IsCode`, `HealthCheck`, `WaitForReady` |
| debug | `transport/debug` | 调试面板（路由/配置/健康/指标/构建信息） | `Handler`, `WithRoutes`, `WithConfig`, `RegisterRoutes` |

### 中间件 → 详见 `middleware` skill

| 模块 | 包路径 | 描述 | 核心类型/函数 |
|------|--------|------|--------------|
| ratelimit | `middleware/ratelimit` | 限流（令牌桶、滑动窗口） | `NewTokenBucket`, `NewSlidingWindow`, `HTTPMiddleware` |
| circuitbreaker | `middleware/circuitbreaker` | 熔断器（Closed/Open/HalfOpen） | `New`, `WithFailureThreshold`, `WithOpenTimeout`, `HTTPMiddleware` |
| retry | `middleware/retry` | 重试（指数退避） | `New`, `WithMaxAttempts`, `WithBackoff` |
| recovery | `middleware/recovery` | Panic 恢复 | `New` |
| timeout | `middleware/timeout` | 超时控制 | `New`, `WithTimeout` |
| cors | `middleware/cors` | 跨域 | `New`, `WithAllowOrigins` |
| idempotency | `middleware/idempotency` | 幂等性保证 | `New`, `WithStore` |
| semaphore | `middleware/semaphore` | 并发控制 | `New`, `WithLimit` |
| logging | `middleware/logging` | 结构化请求日志 | `NewHTTP`, `NewGRPC` |
| secure | `middleware/secure` | 安全头（HSTS/CSP/X-Frame-Options） | `HTTPMiddleware`, `DefaultConfig` |
| csrf | `middleware/csrf` | CSRF 防护（双重提交 Cookie） | `HTTPMiddleware`, `TokenFromContext`, `DefaultConfig` |
| bodylimit | `middleware/bodylimit` | 请求体大小限制 | `HTTPMiddleware`, `ParseLimit` |
| signature | `middleware/signature` | HMAC 请求签名验证 | `HTTPMiddleware`, `DefaultConfig`, `SignRequest`, `Sign`, `Verify` |
| trace | `middleware/trace` | 链路追踪增强（trace-id 传播/日志注入/下游传递） | `HTTPMiddleware`, `GRPCUnaryInterceptor`, `GRPCStreamInterceptor`, `TraceIDFromContext`, `InjectHTTPHeaders`, `InjectGRPCMetadata` |
| gzip | `middleware/gzip` | HTTP 响应 gzip 压缩 | `New`, `Handler`, `WithLevel`, `WithMinLength`, `WithExcludePaths`, `WithExcludeContentTypes` |
| adaptive | `middleware/adaptive` | 自适应限流与降级（CPU/延迟/错误率） | `New`, `Limiter`, `Middleware`, `GRPCUnaryInterceptor`, `RecordLatency`, `RecordError`, `Status` |
| waf | `middleware/waf` | Web 应用防火墙（SQL注入/XSS/路径遍历/命令注入） | `New`, `HTTPMiddleware`, `WithRuleSet`, `WithCustomRules`, `WithMode` |
| version | `middleware/version` | API 版本化（路径/Header 双模式） | `New`, `HTTPMiddleware`, `WithPathPrefix`, `WithHeader`, `WithDefaultVersion` |
| fallback | `middleware/fallback` | 优雅降级（5xx/panic 自动 fallback） | `New`, `HTTPMiddleware`, `WithHandler`, `WithStatusCodes`, `WithTimeout` |
| loadshed | `middleware/loadshed` | 负载卸载（并发/队列深度/延迟阈值） | `New`, `HTTPMiddleware`, `WithMaxConcurrent`, `WithMaxQueueDepth`, `WithLatencyThreshold` |

### 认证 → 详见 `auth` skill

| 模块 | 包路径 | 描述 | 核心类型/函数 |
|------|--------|------|--------------|
| auth/jwt | `auth/jwt` | JWT 签发与验证（HS256/RS256/ES256/EdDSA） | `NewJWT`, `NewAuthenticator`, `WithSecretKey`, `WithRSAKeys`, `WithECDSAKeys`, `WithEdDSAKeys`, `LoadRSAPrivateKey`, `Generate`, `Validate` |
| auth/apikey | `auth/apikey` | API Key 验证 | `New`, `StaticValidator`, `CacheValidator` |
| auth/rbac | `auth/rbac` | 基于角色的访问控制（RBAC） | `NewManager`, `NewMemoryStore`, `NewGORMStore`, `AssignRole`, `HasPermission`, `HTTPMiddleware` |

### 存储 → 详见 `storage` skill

| 模块 | 包路径 | 描述 | 核心类型/函数 |
|------|--------|------|--------------|
| storage/cache | `storage/cache` | 缓存（内存、Redis） | `NewCache`, `NewRedisConfig`, `NewMemoryConfig` |
| storage/rdbms | `storage/rdbms` | 数据库（GORM） | `NewDatabase`, `BaseModel`, `DB`, `AsGORM` |
| storage/mongodb | `storage/mongodb` | MongoDB 客户端 | `NewClient`, `MustNewClient` |
| storage/s3 | `storage/s3` | S3/MinIO 对象存储 | `NewClient`, `MustNewClient` |
| storage/elasticsearch | `storage/elasticsearch` | Elasticsearch 客户端 | `NewClient`, `MustNewClient` |
| storage/lock | `storage/lock` | 分布式锁 | `NewLocker`, `Lock`, `Unlock` |
| storage/sqlx | `storage/sqlx` | sqlx 数据库封装 | `NewDB`, `MustNewDB` |
| storage/migration | `storage/migration` | 数据库迁移（Go DSL） | `NewRegistry`, `Add`, `NewRunner`, `Up`, `Down`, `Status` |
| storage/clickhouse | `storage/clickhouse` | ClickHouse 客户端 | `NewClient`, `MustNewClient`, `DefaultConfig`, `Exec`, `Query`, `Select`, `PrepareBatch` |
| storage/redis | `storage/redis` | Redis 客户端（完整数据类型操作） | `NewClient`, `MustNewClient`, `DefaultConfig`, `Set`, `Get`, `PipelineExec`, `Subscribe` |
| storage/minio | `storage/minio` | MinIO 对象存储 | `NewClient`, `DefaultConfig`, `PutObject`, `GetObject`, `DeleteObject`, `PresignGetObject`, `PresignPutObject` |
| storage/neo4j | `storage/neo4j` | Neo4j 图数据库 | `NewClient`, `DefaultConfig`, `Run`, `ReadTransaction`, `WriteTransaction`, `Close` |

### 可观测性 → 详见 `observability` skill

| 模块 | 包路径 | 描述 | 核心类型/函数 |
|------|--------|------|--------------|
| observability/metrics | `observability/metrics` | Prometheus + OpenTelemetry 指标 | `NewMetrics`, `MustNewMetrics`, `DefaultConfig`, `NewOTel`, `WithMeterProvider`, `WithExporter` |
| observability/tracing | `observability/tracing` | OpenTelemetry 追踪 | `NewTracer`, `TracingConfig`, `OTLPConfig` |
| observability/logger | `observability/logger` | 结构化日志 | `NewLogger`, `NewContext`, `FromContext`, `AsSlog`, `NewFromSlog`, `WithLevel`, `WithOutput` |
| observability/logshipper | `observability/logshipper` | 日志投递（ES/Kafka sink，异步批量） | `New`, `NewElasticsearchSink`, `NewKafkaSink`, `ZapHook`, `AttachToLogger`, `NewLoggerHook` |
| observability/slo | `observability/slo` | SLO/SLI 追踪（错误预算/告警） | `NewTracker`, `Objective`, `Record`, `Status`, `OnBreach`, `PrometheusCollector` |
| observability/alerting | `observability/alerting` | 告警规则引擎（阈值/速率/缺失检测） | `New`, `Engine`, `AddRule`, `RemoveRule`, `Start`, `Stop`, `Evaluate`, `ActiveAlerts`, `AlertHistory` |
| observability/profiling | `observability/profiling` | 持续性能剖析（CPU/内存/Goroutine 周期采集） | `New`, `DefaultConfig`, `Start`, `Stop`, `Collect`, `Handler`, `Status`, `NewFileExporter` |

### 配置与服务发现 → 详见 `config` skill

| 模块 | 包路径 | 描述 | 核心类型/函数 |
|------|--------|------|--------------|
| config | `config` | 多源配置管理 | `NewManager`, `WithSource`, `Load`, `Watch`, `Get` |
| config/source/file | `config/source/file` | 文件配置源（热更新） | `New` |
| config/source/etcd | `config/source/etcd` | etcd 配置源 | `New` |
| config/source/env | `config/source/env` | 环境变量配置源 | `New` |
| config/source/consul | `config/source/consul` | Consul KV 配置源 | `New`, `WithFormat`, `WithDatacenter` |
| config/source/nacos | `config/source/nacos` | Nacos 配置源 | `New`, `WithFormat`, `WithGroup`, `WithNamespace` |
| config/source/apollo | `config/source/apollo` | Apollo 配置中心配置源 | `New`, `Config`, `WithFormat`, `WithCluster`, `WithNamespace` |
| config/source/k8s | `config/source/k8s` | Kubernetes ConfigMap/Secret 配置源 | `New`, `NewWithClient`, `Config`, `WithFormat`, `WithNamespace`, `WithKey`, `WithResourceType` |
| discovery | `discovery` | 服务注册与发现 | `NewDiscovery`, `NewServiceRegistry`, `Register`, `Discover` |

### AI → 详见 `ai` skill

| 模块 | 包路径 | 描述 | 核心类型/函数 |
|------|--------|------|--------------|
| llm/agent/toolcall | `llm/agent/toolcall` | AI 工具调用框架 | `NewRegistry`, `NewExecutor`, `WithOnStep` |
| llm/provider/router | `llm/provider/router` | 多 Provider 路由 | `New`, `Route` |
| llm/provider/openai | `llm/provider/openai` | OpenAI 客户端（兼容 DeepSeek 等） | `New`, `WithBaseURL`, `WithModel`, `WithEmbeddingModel` |
| llm/provider/anthropic | `llm/provider/anthropic` | Anthropic Claude 客户端 | `New`, `WithModel`, `WithDefaultMaxTokens` |
| llm/provider/gemini | `llm/provider/gemini` | Google Gemini 客户端 | `New`, `WithModel`, `WithEmbeddingModel` |
| llm/agent/conversation | `llm/agent/conversation` | 多轮对话会话管理 | `New`, `Chat`, `ChatStream`, `WithMemory`, `NewBufferMemory`, `NewWindowMemory` |
| llm/retrieval/embedding | `llm/retrieval/embedding` | 嵌入向量工具 | `BatchEmbed`, `CosineSimilarity` |
| llm/prompt | `llm/prompt` | 消息模板引擎 | `New`, `MustNew`, `Render`, `MustRender` |
| llm/middleware | `llm/middleware` | AI 模型中间件链 | `Chain`, `Logging`, `Retry`, `RateLimit`, `UsageTracker` |
| llm/retrieval/vectorstore | `llm/retrieval/vectorstore` | 向量存储接口 | `VectorStore`, `Document`, `SimilaritySearch`, `WithFilter` |
| llm/retrieval/splitter | `llm/retrieval/splitter` | 文本分块器（字符/递归/Token） | `NewCharacterSplitter`, `NewRecursiveSplitter`, `NewTokenSplitter`, `WithChunkSize`, `WithChunkOverlap` |
| llm/processing/structured | `llm/processing/structured` | 结构化输出提取（JSON Schema 约束） | `Extract`, `ExtractFromMessages`, `SchemaFrom`, `WithMaxRetries` |
| llm/serving/cache | `llm/serving/cache` | 语义缓存（Embedding 相似度） | `NewMemoryStore`, `Middleware`, `NewCachedModel`, `Config` |
| llm/safety/guardrail | `llm/safety/guardrail` | 输入输出护栏（关键词/PII/长度） | `Middleware`, `KeywordFilter`, `PIIDetector`, `MaxLength`, `WithInputGuards`, `WithOutputGuards` |
| llm/retrieval/rag | `llm/retrieval/rag` | RAG 管线（检索增强生成） | `New`, `Ingest`, `Retrieve`, `Query`, `QueryStream` |
| llm/agent/chain | `llm/agent/chain` | 多步 LLM 编排 | `New`, `AddStep`, `Run`, `WithModel`, `WithOnStep` |
| llm/retrieval/document | `llm/retrieval/document` | 文档加载器（Text/CSV/JSON/Markdown/Directory） | `NewTextLoader`, `NewTextFileLoader`, `NewCSVLoader`, `NewJSONLoader`, `NewMarkdownLoader`, `NewDirectoryLoader` |
| llm/agent/memory | `llm/agent/memory` | 持久化记忆（摘要/实体/Redis/内存） | `NewMemoryStore`, `NewRedisStore`, `NewPersistentMemory`, `NewSummaryMemory`, `NewEntityMemory` |
| llm/retrieval/rerank | `llm/retrieval/rerank` | 重排序器（LLM/Embedding/CrossEncoder） | `NewLLMReranker`, `NewEmbeddingReranker`, `NewCrossEncoderReranker`, `WithTopN` |
| llm/agent | `llm/agent` | 自主 Agent 框架（ReAct/PlanExecute/Supervisor/Pipeline） | `New`, `Run`, `RunStream`, `NewReActStrategy`, `NewPlanExecuteStrategy`, `NewSupervisor`, `NewPipeline` |
| llm/eval | `llm/eval` | LLM 输出评估（相关性/忠实度/连贯性/正确性） | `RelevanceEvaluator`, `FaithfulnessEvaluator`, `CoherenceEvaluator`, `CorrectnessEvaluator`, `NewCompositeEvaluator` |
| llm/processing/tokenizer | `llm/processing/tokenizer` | Token 计数与截断（估算/CL100K/消息列表） | `NewEstimateTokenizer`, `NewCL100KTokenizer`, `EstimateTokens`, `TruncateToFit`, `FitsContext` |
| llm/safety/moderation | `llm/safety/moderation` | 内容审核（LLM/关键词/组合审核） | `NewLLMModerator`, `NewKeywordModerator`, `NewCompositeModerator`, `WithThreshold`, `WithCategories` |
| llm/serving/apikey | `llm/serving/apikey` | API Key 管理（签发/验证/配额/限流） | `NewManager`, `NewGORMStore`, `NewMemoryStore`, `HTTPMiddleware`, `FromContext`, `WithQuotaLimit`, `WithRateLimit` |
| llm/serving/billing | `llm/serving/billing` | 用量计费（按 token 计费/用量报表） | `NewBilling`, `NewGORMStore`, `NewMemoryStore`, `Middleware`, `WithDefaultPricing`, `SetPricing`, `GetSummary` |
| llm/serving/proxy | `llm/serving/proxy` | AI API 代理网关（OpenAI 兼容/路由/鉴权/计费） | `New`, `RegisterProvider`, `Route`, `Handler`, `WithAPIKeyManager`, `WithBilling`, `WithModeration` |
| llm/processing/classifier | `llm/processing/classifier` | 文本分类器（意图/情感/主题/语言/毒性/路由） | `NewIntentClassifier`, `NewSentimentClassifier`, `NewTopicClassifier`, `NewLanguageClassifier`, `NewToxicityClassifier`, `NewRouterClassifier`, `NewCustomClassifier`, `WithTopN` |
| llm/processing/extractor | `llm/processing/extractor` | 信息提取（实体/关系/关键词/摘要） | `NewEntityExtractor`, `NewRelationExtractor`, `NewKeywordExtractor`, `NewSummarizer`, `WithMaxKeywords`, `WithMaxSentences`, `WithLanguage` |
| llm/processing/translator | `llm/processing/translator` | 翻译器（多语言/术语表/批量翻译） | `NewTranslator`, `Translate`, `TranslateBatch`, `DetectLanguage`, `WithSourceLanguage`, `WithGlossary`, `WithTone`, `WithBatchSize` |

### 分布式模式 → 详见 `distributed` skill

| 模块 | 包路径 | 描述 | 核心类型/函数 |
|------|--------|------|--------------|
| domain/cqrs | `domain/cqrs` | CQRS 命令/查询模式 | `ChainCommand`, `ApplyCommand`, `ChainQuery`, `ApplyQueryHandler` |
| domain/outbox | `domain/outbox` | Outbox 事务消息 | `NewGORMStore`, `NewRelay`, `InjectTx`, `WithTx` |
| domain | `domain` | 领域事件总线 | `NewEventBus`, `NewAsyncEventBus`, `NewJSONEventConverter` |
| domain/saga | `domain/saga` | Saga 分布式事务编排 | `NewSaga`, `Step`, `Compensate`, `Execute` |
| domain/eventsourcing | `domain/eventsourcing` | 事件溯源 | `NewBaseAggregate`, `RaiseEvent`, `NewRepository`, `NewGORMEventStore`, `WithSnapshotStore` |

### 消息与任务 → 详见 `pubsub` skill

| 模块 | 包路径 | 描述 | 核心类型/函数 |
|------|--------|------|--------------|
| pubsub | `messaging/pubsub` | 统一 Pub/Sub 接口 | `Publisher`, `Subscriber`, `Message` |
| pubsub/kafka | `messaging/pubsub/kafka` | Kafka driver | `NewPublisher`, `NewSubscriber` |
| pubsub/rabbitmq | `messaging/pubsub/rabbitmq` | RabbitMQ driver | `NewPublisher`, `NewSubscriber` |
| pubsub/redis | `messaging/pubsub/redis` | Redis Streams driver | `NewPublisher`, `NewSubscriber` |
| jobqueue | `messaging/jobqueue` | 异步任务队列 | `NewClient`, `NewWorker`, `Store` |
| jobqueue/redis | `messaging/jobqueue/redis` | Redis Store | `NewStore` |
| jobqueue/kafka | `messaging/jobqueue/kafka` | Kafka Store | `NewStore` |
| jobqueue/rabbitmq | `messaging/jobqueue/rabbitmq` | RabbitMQ Store | `NewStore` |
| jobqueue/database | `messaging/jobqueue/database` | GORM Database Store | `NewStore` |
| eventbus | `messaging/eventbus` | 进程内事件总线（同步/异步分发） | `New`, `Bus`, `Subscribe`, `SubscribeAll`, `Publish`, `PublishAsync`, `Close` |

### 通知

| 模块 | 包路径 | 描述 | 核心类型/函数 |
|------|--------|------|--------------|
| notify | `notify` | 通知发送统一接口 | `Sender`, `Message`, `SendOption` |
| notify/email | `notify/email` | 邮件通知 | `NewSender`, `WithSMTP`, `WithFrom` |
| notify/sms | `notify/sms` | 短信通知 | `NewSender`, `WithProvider` |
| notify/push | `notify/push` | 推送通知（APNs/FCM） | `NewSender`, `WithAPNs`, `WithFCM` |
| notify/nwebhook | `notify/nwebhook` | Webhook 通知发送 | `NewSender`, `WithURL`, `WithSigner` |
| notify/webhook | `notify/webhook` | Webhook 投递与接收 | `NewDispatcher`, `NewReceiver`, `NewHMACSigner` |
| notify/webhook/store/memory | `notify/webhook/store/memory` | 内存 SubscriptionStore | `NewStore` |
| notify/webhook/store/gorm | `notify/webhook/store/gorm` | GORM SubscriptionStore | `NewStore` |

### OAuth2 → 详见 `oauth2` skill

| 模块 | 包路径 | 描述 | 核心类型/函数 |
|------|--------|------|--------------|
| oauth2 | `oauth2` | Provider/StateStore 接口 | `Provider`, `StateStore`, `Token`, `UserInfo` |
| oauth2/state | `oauth2/state` | State 管理 | `NewMemoryStore`, `NewRedisStore` |
| oauth2/github | `oauth2/github` | GitHub OAuth2 | `NewProvider` |
| oauth2/google | `oauth2/google` | Google OAuth2 | `NewProvider` |
| oauth2/wechat | `oauth2/wechat` | 微信 OAuth2 | `NewProvider` |

### OpenAPI → 详见 `openapi` skill

| 模块 | 包路径 | 描述 | 核心类型/函数 |
|------|--------|------|--------------|
| openapi | `openapi` | Code-first OpenAPI 3.1 生成（含 Webhooks） | `NewRegistry`, `SchemaFrom`, `GET/POST/PUT/DELETE/PATCH`, `ServeJSON`, `ServeYAML`, `AddWebhook` |

### 请求上下文

| 模块 | 包路径 | 描述 | 核心类型/函数 |
|------|--------|------|--------------|
| httpx | `httpx` | HTTP 请求上下文工具（clientip/useragent/botdetect/locale 等） | `HTTPMiddleware`, `GRPCInterceptor` |

### 错误处理

| 模块 | 包路径 | 描述 | 核心类型/函数 |
|------|--------|------|--------------|
| errors | `errors` | 统一业务错误（含 HTTP/gRPC 状态映射） | `New`, `Error`, `WithHTTP`, `WithGRPC`, `FromHTTPStatus`, `FromGRPCCode` |

### 输入校验

| 模块 | 包路径 | 描述 | 核心类型/函数 |
|------|--------|------|--------------|
| validation | `validation` | 输入校验（中英文错误消息、自定义规则） | `New`, `Validate`, `ValidateField`, `RegisterValidation`, `ParseErrors`, `WithLocale`, `WithMessages` |

### 国际化

| 模块 | 包路径 | 描述 | 核心类型/函数 |
|------|--------|------|--------------|
| i18n | `i18n` | 国际化本地化 | `NewBundle`, `LoadFiles`, `Translate`, `WithLogger` |

### 多租户

| 模块 | 包路径 | 描述 | 核心类型/函数 |
|------|--------|------|--------------|
| tenant | `tenant` | 多租户支持 | `Tenant`, `WithTenant`, `FromContext`, `ID`, `HTTPMiddleware`, `Resolver`, `KeySpace` |

### 工具库

| 模块 | 包路径 | 描述 | 核心类型/函数 |
|------|--------|------|--------------|
| xutil/pagination | `xutil/pagination` | 分页工具 | `Params`, `Result`, `Paginate` |
| xutil/crypto | `xutil/crypto` | 加密工具 | `HashPassword`, `ComparePassword` |
| xutil/ptrx | `xutil/ptrx` | 指针工具 | `Of`, `Value`, `ValueOr` |
| xutil/optionx | `xutil/optionx` | 选项模式工具 | `Option`, `Apply` |
| xutil/version | `xutil/version` | 版本信息 | `Version`, `Print` |
| xutil/sorting | `xutil/sorting` | 排序工具 | `Sort`, `GORMScope` |
| xutil/copier | `xutil/copier` | 结构体拷贝 | `Copy`, `CopySlice` |
| xutil/syncx | `xutil/syncx` | 并发工具 | `Map`, `Pool`, `LimitPool`, `SegmentKeysLock` |
| xutil/strx | `xutil/strx` | 字符串工具 | 字符串处理函数集 |
| xutil/randx | `xutil/randx` | 随机数工具 | 随机数/字符串生成 |
| xutil/iox | `xutil/iox` | IO 工具 | IO 辅助函数 |
| xutil/valuex | `xutil/valuex` | 值工具 | `AnyValue` 类型安全取值 |
| xutil/idgen | `xutil/idgen` | 分布式 ID 生成 | `NewSnowflake`, `NewULID`, `NewNanoID`, `Snowflake`, `ULID`, `NanoID`, `UUID` |
| xutil/templatex | `xutil/templatex` | 增强模板引擎（text/html 双引擎+内置函数） | `New`, `Engine`, `Render`, `RenderString`, `RenderHTML`, `ParseFile`, `ParseGlob` |

### 业务组件 → 详见 `bizx` skill

| 模块 | 包路径 | 描述 | 核心类型/函数 |
|------|--------|------|--------------|
| bizx/counter | `bizx/counter` | 分布式计数器（精确计数/滑动窗口） | `NewMemoryCounter`, `NewRedisCounter`, `WithPrefix` |
| bizx/leaderboard | `bizx/leaderboard` | 排行榜（Top N/排名/分页） | `NewMemoryLeaderboard`, `NewRedisLeaderboard`, `WithOrder` |
| bizx/sequence | `bizx/sequence` | 业务序号生成（ORD-20260405-0001） | `New`, `NewMemoryStore`, `NewRedisStore`, `Config` |
| bizx/locking | `bizx/locking` | 业务锁（可重入/读写锁/续期） | `NewLock`, `NewReentrantLock`, `NewRWLock`, `WithLock`, `WithRLock` |
| bizx/ratelimit | `bizx/ratelimit` | 业务配额（按用户/租户限流） | `NewMemoryQuotaManager`, `NewRedisQuotaManager`, `Quota`, `Usage` |
| bizx/statemachine | `bizx/statemachine` | 状态机（状态/事件/守卫/回调） | `New`, `Fire`, `Current`, `Can`, `OnEnter`, `OnLeave`, `OnTransition` |
| bizx/pagination | `bizx/pagination` | 游标分页（Cursor-based） | `GORMPaginate`, `EncodeCursor`, `DecodeCursor`, `CursorRequest`, `CursorResponse` |
| bizx/audit | `bizx/audit` | 审计日志（操作记录/变更追踪） | `NewLogger`, `NewGORMStore`, `HTTPMiddleware`, `Entry`, `Change` |
| bizx/feature | `bizx/feature` | 特性开关（灰度/百分比/白名单） | `NewManager`, `NewMemoryStore`, `NewRedisStore`, `Flag`, `WithUser`, `WithGroup` |
| bizx/retry | `bizx/retry` | 异步重试（持久化/指数退避/死信） | `NewScheduler`, `NewGORMStore`, `Register`, `Submit`, `WithMaxRetries` |
| bizx/event | `bizx/event` | 进程内事件总线（通配符/优先级/异步） | `New`, `Publish`, `Subscribe`, `Unsubscribe`, `WithPriority`, `WithAsync` |
| bizx/captcha | `bizx/captcha` | 验证码管理（生成/验证/防刷/冷却） | `NewManager`, `NewRedisStore`, `Generate`, `Verify`, `WithCooldown` |
| bizx/workflow | `bizx/workflow` | 工作流引擎（审批/条件/并行） | `New`, `Engine`, `RegisterDefinition`, `StartWorkflow`, `Execute`, `Approve`, `Reject`, `Cancel` |
| bizx/abtesting | `bizx/abtesting` | A/B 测试（流量分桶/多变体/曝光追踪） | `New`, `Manager`, `CreateExperiment`, `Assign`, `TrackExposure`, `NewMemoryStore` |

### 测试工具 → 详见 `testx` skill

| 模块 | 包路径 | 描述 | 核心类型/函数 |
|------|--------|------|--------------|
| testx | `testx` | 测试工具包（NopLogger/TestLogger/Container/HTTPTest/Fixture） | `NopLogger`, `TestLogger`, `NewRedis`, `NewPostgres`, `NewHTTPTestServer`, `NewGRPCTestServer`, `Golden`, `LoadJSON` |

## 维护说明

当 servex 公开 API 发生变更时：

1. 更新本文件对应模块的索引表格
2. 在对应分组 skill 文件（`servex-transport.md` 等）中修正代码示例
3. 在 `docs/superpowers/examples/` 对应目录修正完整示例
4. 运行 `go -C docs/superpowers/examples build ./...` 确认编译通过
