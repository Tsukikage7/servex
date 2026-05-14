[![CI](https://github.com/Tsukikage7/servex/actions/workflows/ci.yml/badge.svg)](https://github.com/Tsukikage7/servex/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/Tsukikage7/servex)](https://goreportcard.com/report/github.com/Tsukikage7/servex)
[![Go Reference](https://pkg.go.dev/badge/github.com/Tsukikage7/servex.svg)](https://pkg.go.dev/github.com/Tsukikage7/servex)
[![codecov](https://codecov.io/gh/Tsukikage7/servex/branch/main/graph/badge.svg)](https://codecov.io/gh/Tsukikage7/servex)

# servex

Go 微服务开发工具包，提供构建生产级微服务所需的核心组件。

## 安装

```bash
go get github.com/Tsukikage7/servex/v2
```

## 按需依赖

servex 的基础包默认只暴露抽象和轻量实现，Redis、Consul、etcd、Nacos、Kafka、RabbitMQ、GORM、gRPC adapter、testcontainers 等重依赖放在子包中按需导入。常见拆分路径如下：

| 能力 | 基础包 | 按需子包 |
| --- | --- | --- |
| 错误模型 | `errors` | `errors/grpcx` |
| 认证抽象 | `auth` | `auth/grpcx` |
| JWT | `auth/jwt` | `auth/jwt/grpcx` |
| 通知 | `notify` | `notify/jobqueuex` |
| 缓存 | `storage/cache` | `storage/cache/redis` |
| 服务发现 | `discovery` | `discovery/consul`、`discovery/etcd`、`discovery/nacos` |
| Pub/Sub 工厂 | `messaging/pubsub/factory` | `messaging/pubsub/factory/redis`、`kafka`、`rabbitmq` |
| 分页/排序 | `xutil/pagination`、`xutil/sorting` | `xutil/pagination/gorm`、`xutil/sorting/gorm` |
| 测试工具 | `testx` | `testx/grpcx`、`testx/container` |

使用 Config 工厂时，provider 子包通过 blank import 注册：

```go
import (
    "github.com/Tsukikage7/servex/v2/discovery"
    _ "github.com/Tsukikage7/servex/v2/discovery/consul"
)
```

本仓库用 `just deps-check` 固化这些边界，防止基础包重新引入未使用的重依赖。

## Claude Code Plugin

servex 内置 [Claude Code Plugin](https://code.claude.com/docs/en/plugins.md)，为 AI 辅助开发提供模块使用指南、代码生成规范和最佳实践。

**安装插件：**

```bash
# 1. 添加 servex marketplace
/plugin marketplace add Tsukikage7/servex

# 2. 安装插件
/plugin install servex@servex
```

也可以在 Claude Code 中输入 `/plugin`，在 Marketplaces 标签页手动添加。

**Skills：**

| Skill | 说明 |
|-------|------|
| `servex:servex` | 主入口 -- 模块索引、代码规范、工作流，按需加载 20 个子模块参考文档 |
| `servex:llm` | LLM 模块 -- facade、Provider、Middleware、Serving、Eino/ADK 封装 |

安装后 Claude 会根据上下文自动触发，也可手动调用 `/servex:servex` 或 `/servex:llm`。子模块详细文档由主 skill 按需读取，不会污染 skill 列表。

## CLI 工具

servex 提供基于 [cobra](https://github.com/spf13/cobra) 的脚手架 CLI，对标 kratos/goctl，支持交互式向导（[charmbracelet/huh](https://github.com/charmbracelet/huh)，Everforest Dark 主题）。

### 安装

```bash
go install github.com/Tsukikage7/servex/v2/cmd/servex@latest
```

### 交互式向导

```bash
# 无参数自动启动交互式向导
servex new
servex add service
```

### 命令行模式（CI/脚本）

```bash
# 创建 monorepo 项目（默认）
servex new myproject --module github.com/example/myproject

# 创建独立单服务项目
servex new myservice --standalone --with-grpc --infra "mysql,redis"

# 添加微服务
servex add service order --with-grpc --with-gateway \
  --infra "mysql,redis,kafka" \
  --observe "metrics,tracing" \
  --auth "jwt" \
  --discovery "consul"

# 生成 DDD 聚合（业务语义）
servex add aggregate order \
  --fields "id:uint64,user_id:uint64,status:string,total:float64" \
  --commands "Place,Cancel,Ship" \
  --unique "user_id" \
  --service order

# 生成子实体和值对象
servex gen entity order_item --aggregate order --fields "id:uint64,product_id:uint64,quantity:int"
servex gen valueobject address --aggregate order --fields "street:string,city:string,zip:string"

# 生成外部服务适配器（防腐层）
servex gen client user --service order

# Proto 管理（基于 buf）
servex proto add order
servex proto client api/order/v1/order.proto
servex proto server api/order/v1/order.proto --service order
servex proto lint api/
servex proto breaking api/ --against ".git#branch=main"

# Dockerfile / justfile
servex gen dockerfile --name myservice --port 8080
servex gen justfile --name myservice

# 开发模式（文件变更自动重启）
servex dev

# K8s manifest 生成
servex gen k8s --name myservice --port 8080 --replicas 3 --image myservice:latest

# 运行 / 升级 / 补全
servex run
servex upgrade
servex completion bash/zsh/fish
```

### 可用基础设施组件

| Flag | 可选值 |
|------|--------|
| --infra | mysql, postgres, sqlite, redis, mongo, es, clickhouse, s3, minio, neo4j, kafka, rabbitmq |
| --observe | metrics, tracing, profiling |
| --auth | jwt |
| --discovery | consul, etcd, nacos |
| --other | scheduler, i18n, tenant |

### 生成的项目结构（monorepo + 六边形架构）

```
myproject/
├── domain/              # 共享领域层
│   └── order/
│       ├── aggregate.go  # 聚合根 + Reconstruct
│       ├── event.go      # 业务事件（OrderPlaced/Cancelled/...）
│       ├── repository.go # 仓储接口 + Filter
│       ├── command.go    # CQRS 业务命令
│       ├── query.go      # 查询 + View
│       └── ports.go      # 防腐层接口
├── application/         # 共享应用服务
│   └── order/service.go # EventBus + DTO 返回
├── services/
│   └── order-service/
│       ├── cmd/server/main.go
│       ├── internal/
│       │   ├── port/         # 入站端口
│       │   │   ├── http.go   # Router + Handle
│       │   │   ├── grpc.go   # RegisterGRPC
│       │   │   └── gateway.go # HTTP+gRPC 双协议
│       │   ├── adapter/      # 出站适配器
│       │   │   ├── persistence/  # DB 仓储
│       │   │   └── external/     # 外部服务客户端
│       │   └── service/      # proto 服务实现
│       └── configs/config.yaml
├── api/                 # 共享 Proto
├── justfile
└── go.mod
```

## 包概览

### 核心

| 包 | 说明 |
| --- | --- |
| [app](./app/) | 应用生命周期管理 |
| [endpoint](./endpoint/) | Endpoint / Middleware 核心抽象 |
| [errors](./errors/) | 统一业务错误（HTTP 映射；gRPC 适配见 `errors/grpcx`） |
| [encoding](./encoding/) | 编解码器接口与 HTTP 内容协商（json/proto/xml/pbjson） |

### 传输层 (transport/)

| 包 | 说明 |
| --- | --- |
| [transport/httpserver](./transport/httpserver/) | HTTP 服务器（Config 驱动、pprof、Recovery、中间件） |
| [transport/grpcserver](./transport/grpcserver/) | gRPC 服务器 |
| [transport/httpclient](./transport/httpclient/) | HTTP 客户端（Config 驱动、retry/circuitbreaker/tracing/metrics 内置中间件） |
| [transport/grpcclient](./transport/grpcclient/) | gRPC 客户端（服务发现/重试/熔断/追踪/负载均衡） |
| [transport/gateway](./transport/gateway/) | API 网关（gRPC+HTTP 双协议/CORS/限流/追踪/认证） |
| [transport/connectserver](./transport/connectserver/) | Connect RPC 服务器（选配，面向 Connect-Web/gRPC-Web） |
| [transport/ginserver](./transport/ginserver/) | Gin 适配 |
| [transport/echoserver](./transport/echoserver/) | Echo 适配 |
| [transport/hertzserver](./transport/hertzserver/) | Hertz 适配 |
| [transport/websocket](./transport/websocket/) | WebSocket 服务端（gorilla/websocket） |
| [transport/sse](./transport/sse/) | Server-Sent Events 服务端 |
| [transport/botserver](./transport/botserver/) | Bot 框架（Discord/Telegram，平台无关路由） |
| [transport/health](./transport/health/) | 健康检查（K8s 探针） |
| [transport/response](./transport/response/) | 统一响应封装 |
| [transport/graphql](./transport/graphql/) | GraphQL 服务器适配（graphql-go/graphql） |
| [transport/tls](./transport/tls/) | TLS 配置工具（证书/mTLS/版本控制） |
| [transport/grpcx](./transport/grpcx/) | gRPC 工具包（流包装/Metadata/错误/健康检查） |
| [transport/debug](./transport/debug/) | 调试面板（路由/配置/健康/指标/构建信息） |

### 中间件 (middleware/)

| 包 | 说明 | Endpoint | HTTP | gRPC |
| --- | --- | :---: | :---: | :---: |
| [middleware/ratelimit](./middleware/ratelimit/) | 限流（令牌桶、滑动窗口、分布式） | Y | Y | Y |
| [middleware/circuitbreaker](./middleware/circuitbreaker/) | 熔断器（Closed/Open/HalfOpen） | Y | Y | Y |
| [middleware/retry](./middleware/retry/) | 重试机制（指数退避） | Y | Y | Y |
| [middleware/recovery](./middleware/recovery/) | Panic 恢复 | Y | Y | Y |
| [middleware/timeout](./middleware/timeout/) | 超时控制 | Y | Y | Y |
| [middleware/cors](./middleware/cors/) | 跨域资源共享（CORS） | - | Y | - |
| [middleware/idempotency](./middleware/idempotency/) | 幂等性保证 | Y | Y | - |
| [middleware/semaphore](./middleware/semaphore/) | 并发控制 | Y | - | - |
| [middleware/logging](./middleware/logging/) | 请求日志（HTTP / gRPC） | - | Y | Y |
| [middleware/secure](./middleware/secure/) | 安全头（HSTS/CSP/X-Frame-Options） | - | Y | - |
| [middleware/csrf](./middleware/csrf/) | CSRF 防护（双重提交 Cookie） | - | Y | - |
| [middleware/bodylimit](./middleware/bodylimit/) | 请求体大小限制 | - | Y | - |
| [middleware/signature](./middleware/signature/) | HMAC 请求签名验证（防重放） | - | Y | - |
| [middleware/trace](./middleware/trace/) | 链路追踪增强（trace-id 传播/日志注入/下游传递） | - | Y | Y |
| [middleware/gzip](./middleware/gzip/) | gzip 响应压缩 | - | Y | - |
| [middleware/adaptive](./middleware/adaptive/) | 自适应限流/降级（CPU/延迟/错误率） | - | Y | Y |
| [middleware/waf](./middleware/waf/) | Web 应用防火墙（SQL注入/XSS/路径遍历/命令注入） | - | Y | - |
| [middleware/version](./middleware/version/) | API 版本化（路径/Header 双模式） | - | Y | - |
| [middleware/fallback](./middleware/fallback/) | 优雅降级（5xx/panic 自动 fallback） | - | Y | - |
| [middleware/loadshed](./middleware/loadshed/) | 负载卸载（并发/队列深度/延迟阈值） | - | Y | - |

### 认证 (auth/)

| 包 | 说明 |
| --- | --- |
| [auth/jwt](./auth/jwt/) | JWT 认证（HS256/RS256/ES256/EdDSA/签发/验证/claims） |
| [auth/apikey](./auth/apikey/) | API Key 认证 |
| [auth/proto](./auth/proto/) | Protobuf 声明式认证与授权策略 |
| [auth/rbac](./auth/rbac/) | 可选 RBAC 授权适配 |
| [auth/casbin](./auth/casbin/) | 可选 Casbin 授权适配 |

### 可观测性 (observability/)

| 包 | 说明 | Endpoint | HTTP | gRPC |
| --- | --- | :---: | :---: | :---: |
| [observability/logger](./observability/logger/) | 结构化日志（Zap） | - | - | - |
| [observability/metrics](./observability/metrics/) | Prometheus + OpenTelemetry 指标收集 | Y | Y | Y |
| [observability/tracing](./observability/tracing/) | OpenTelemetry 链路追踪 | Y | Y | Y |
| [observability/logshipper](./observability/logshipper/) | 日志投递（ES/Kafka sink，异步批量） | - | - | - |
| [observability/slo](./observability/slo/) | SLO/SLI 追踪（错误预算/燃烧率/告警） | - | - | - |
| [observability/alerting](./observability/alerting/) | 告警规则引擎（阈值/速率/缺失检测） | - | - | - |
| [observability/profiling](./observability/profiling/) | 持续性能剖析（CPU/内存/Goroutine 周期采集） | - | - | - |

### 配置与服务发现

| 包 | 说明 |
| --- | --- |
| [config](./config/) | 配置管理（多源热加载、Source 抽象） |
| [config/source/file](./config/source/file/) | 文件配置源 |
| [config/source/etcd](./config/source/etcd/) | etcd 配置源 |
| [config/source/consul](./config/source/consul/) | Consul 配置源 |
| [config/source/env](./config/source/env/) | 环境变量配置源 |
| [config/source/nacos](./config/source/nacos/) | Nacos 配置源（监听变更） |
| [config/source/apollo](./config/source/apollo/) | Apollo 配置中心（变更监听） |
| [config/source/k8s](./config/source/k8s/) | Kubernetes ConfigMap/Secret 配置源 |
| [discovery](./discovery/) | 服务发现抽象与工厂（Consul/etcd/Nacos 按需注册） |

### 存储 (storage/)

| 包 | 说明 | 工厂函数 |
| --- | --- | --- |
| [storage/cache](./storage/cache/) | 缓存抽象与内存实现（Redis 按需注册） | `NewCache` / `MustNewCache` |
| [storage/rdbms](./storage/rdbms/) | 关系数据库（GORM） | `NewDatabase` / `MustNewDatabase` |
| [storage/mongodb](./storage/mongodb/) | MongoDB 客户端 | `NewClient` / `MustNewClient` |
| [storage/elasticsearch](./storage/elasticsearch/) | Elasticsearch 客户端 | `NewClient` / `MustNewClient` |
| [storage/s3](./storage/s3/) | S3 兼容对象存储 | `NewClient` / `MustNewClient` |
| [storage/lock](./storage/lock/) | 分布式锁 | `NewLock` |
| [storage/sqlx](./storage/sqlx/) | sqlx 封装 | `NewDB` |
| [storage/migration](./storage/migration/) | 数据库迁移（Go DSL） | `NewRegistry` / `NewRunner` |
| [storage/clickhouse](./storage/clickhouse/) | ClickHouse 客户端 | `NewClient` / `MustNewClient` |
| [storage/redis](./storage/redis/) | Redis 客户端（完整数据类型 + Pipeline + Pub/Sub） | `NewClient` / `MustNewClient` |
| [storage/minio](./storage/minio/) | MinIO 对象存储客户端（原生 SDK） | `NewClient` / `MustNewClient` |
| [storage/neo4j](./storage/neo4j/) | Neo4j 图数据库客户端 | `NewClient` / `MustNewClient` |

### 消息 (messaging/)

| 包 | 说明 | 工厂函数 |
| --- | --- | --- |
| [messaging/pubsub](./messaging/pubsub/) | 统一 Pub/Sub 抽象 | - |
| [messaging/pubsub/factory](./messaging/pubsub/factory/) | Config 驱动工厂（后端按需注册） | `NewPublisher` / `NewSubscriber` |
| [messaging/pubsub/kafka](./messaging/pubsub/kafka/) | Kafka driver | `NewPublisher` / `NewSubscriber` |
| [messaging/pubsub/rabbitmq](./messaging/pubsub/rabbitmq/) | RabbitMQ driver | `NewPublisher` / `NewSubscriber` |
| [messaging/pubsub/redis](./messaging/pubsub/redis/) | Redis Streams driver | `NewPublisher` / `NewSubscriber` |
| [messaging/jobqueue](./messaging/jobqueue/) | 异步任务队列（延迟、优先级、重试、死信） | `NewClient` / `NewWorker` |
| [messaging/jobqueue/factory](./messaging/jobqueue/factory/) | Config 驱动工厂（后端按需注册） | `NewStore` |
| [messaging/jobqueue/redis](./messaging/jobqueue/redis/) | Redis Store | `NewStore` |
| [messaging/jobqueue/kafka](./messaging/jobqueue/kafka/) | Kafka Store | `NewStore` |
| [messaging/jobqueue/rabbitmq](./messaging/jobqueue/rabbitmq/) | RabbitMQ Store | `NewStore` |
| [messaging/eventbus](./messaging/eventbus/) | 进程内事件总线（同步/异步分发） | - |

### 领域驱动 (domain/)

| 包 | 说明 |
| --- | --- |
| [domain](./domain/) | 聚合根、领域事件、EventBus |
| [domain/cqrs](./domain/cqrs/) | 命令查询职责分离 |
| [domain/saga](./domain/saga/) | Saga 分布式事务 |
| [domain/outbox](./domain/outbox/) | 事务发件箱模式 |
| [domain/eventsourcing](./domain/eventsourcing/) | 事件溯源（Event Sourcing） |

### 通知 (notify/)

| 包 | 说明 |
| --- | --- |
| [notify](./notify/) | 统一通知接口 |
| [notify/email](./notify/email/) | 邮件通知 |
| [notify/sms](./notify/sms/) | 短信通知 |
| [notify/push](./notify/push/) | 推送通知 |
| [notify/webhook](./notify/webhook/) | Webhook 投递与接收 |
| [notify/nwebhook](./notify/nwebhook/) | Webhook 通知渠道 |
| [notify/factory](./notify/factory/) | 通知渠道工厂 |
| [notify/jobqueuex](./notify/jobqueuex/) | JobQueue 异步通知投递适配 |

### HTTP 请求分析 (httpx/)

| 包 | 说明 |
| --- | --- |
| [httpx](./httpx/) | 组合中间件（统一入口） |
| [httpx/clientip](./httpx/clientip/) | 客户端 IP 提取、地理位置、ACL |
| [httpx/useragent](./httpx/useragent/) | User-Agent 解析 |
| [httpx/deviceinfo](./httpx/deviceinfo/) | 设备信息（Client Hints 优先） |
| [httpx/botdetect](./httpx/botdetect/) | 机器人检测 |
| [httpx/locale](./httpx/locale/) | 语言区域设置 |
| [httpx/referer](./httpx/referer/) | 来源页面解析、UTM 参数 |
| [httpx/activity](./httpx/activity/) | 用户活动追踪（Redis + Kafka） |

### AI 集成 (llm/)

| 包 | 说明 |
| --- | --- |
| [llm](./llm/) | 统一 ChatModel / EmbeddingModel 接口抽象 |
| [llm/framework/eino](./llm/framework/eino/) | 独立 module，CloudWeGo Eino 封装（消息、工具、ChatModel、EmbeddingModel 双向适配） |
| [llm/framework/adk](./llm/framework/adk/) | 独立 module，Google ADK 封装（Agent、LLMAgent、Runner、model.LLM 适配） |
| [llm/provider/openai](./llm/provider/openai/) | OpenAI 适配器（兼容 DeepSeek、通义千问等） |
| [llm/provider/anthropic](./llm/provider/anthropic/) | Anthropic Claude 适配器 |
| [llm/provider/gemini](./llm/provider/gemini/) | Google Gemini 适配器 |
| [llm/provider/ollama](./llm/provider/ollama/) | Ollama 本地模型适配器 |
| [llm/provider/deepseek](./llm/provider/deepseek/) | DeepSeek 适配器 |
| [llm/provider/bedrock](./llm/provider/bedrock/) | AWS Bedrock 适配器（Converse API，支持 Claude/Titan/Llama） |
| [llm/provider/router](./llm/provider/router/) | 多 Provider 路由器（按模型名路由） |
| [llm/prompt](./llm/prompt/) | 基于 text/template 的提示词模板引擎 |
| [llm/middleware](./llm/middleware/) | AI 中间件链（日志、重试、限流、用量追踪） |
| [llm/serving/cache](./llm/serving/cache/) | 语义缓存（Embedding 相似度） |
| [llm/serving/apikey](./llm/serving/apikey/) | API Key 管理（签发/验证/配额/限流） |
| [llm/serving/billing](./llm/serving/billing/) | 用量计费（按 token 计费/用量报表） |
| [llm/serving/proxy](./llm/serving/proxy/) | AI API 代理网关（OpenAI 兼容/路由/Fallback） |

### OAuth2 第三方登录

| 包 | 说明 |
| --- | --- |
| [oauth2](./oauth2/) | Provider / StateStore 接口 |
| [oauth2/github](./oauth2/github/) | GitHub OAuth2 Provider |
| [oauth2/google](./oauth2/google/) | Google OAuth2 Provider |
| [oauth2/wechat](./oauth2/wechat/) | 微信 OAuth2 Provider |
| [oauth2/state](./oauth2/state/) | State 管理（Memory / Redis） |

### 其他

| 包 | 说明 |
| --- | --- |
| [openapi](./openapi/) | Code-first OpenAPI 3.1 生成（含 Webhooks） |
| [scheduler](./scheduler/) | Cron 定时任务调度 |
| [i18n](./i18n/) | 国际化 |
| [tenant](./tenant/) | 多租户（GORM Scope） |
| [collections](./collections/) | 数据结构（Deque/LRU/TreeMap/PriorityQueue/HashSet 等，12 子包） |
| [xutil](./xutil/) | 工具包（ptrx/strx/randx/iox/copier/syncx/sorting/pagination/version/crypto/optionx/valuex/idgen） |
| [xutil/templatex](./xutil/templatex/) | 模板引擎增强（14 个内置函数/多格式） |
| [validation](./validation/) | 输入校验（go-playground/validator 封装，中英文错误消息） |
| [testx](./testx/) | 轻量测试工具包（NopLogger/TestLogger/HTTPTest/Fixture；容器和 gRPC 见子包） |

### 业务组件 (bizx/)

| 包 | 说明 |
| --- | --- |
| [bizx/counter](./bizx/counter/) | 分布式计数器（精确计数/滑动窗口） |
| [bizx/leaderboard](./bizx/leaderboard/) | 排行榜（Top N/排名/分页） |
| [bizx/sequence](./bizx/sequence/) | 业务序号生成（ORD-20260405-0001） |
| [bizx/locking](./bizx/locking/) | 业务锁（可重入/读写锁/续期） |
| [bizx/ratelimit](./bizx/ratelimit/) | 业务配额（按用户/租户限流） |
| [bizx/statemachine](./bizx/statemachine/) | 状态机（状态/事件/守卫/回调） |
| [bizx/audit](./bizx/audit/) | 审计日志（操作记录/变更追踪） |
| [bizx/feature](./bizx/feature/) | 特性开关（灰度/百分比/白名单） |
| [bizx/retry](./bizx/retry/) | 异步重试（持久化/指数退避/死信） |
| [bizx/event](./bizx/event/) | 进程内事件总线（通配符/优先级/异步） |
| [bizx/captcha](./bizx/captcha/) | 验证码管理（生成/验证/防刷/冷却） |
| [bizx/workflow](./bizx/workflow/) | 工作流引擎（审批/条件分支/并行执行） |
| [bizx/abtesting](./bizx/abtesting/) | A/B 测试（流量分桶/多变体/曝光追踪） |

## v2.1.x 新特性

### v2.1.1

- **错误体系统一** — `transport/response` 与 `errors` 包互通，`Code.ToError()` 一键转换为统一错误类型
- **日志分级分目录** — `observability/logger` 支持 `LevelSeparate`，按天/小时轮转，目录格式 `prefix/YYYYMMDD/prefix.log` 或 `prefix/YYYYMMDDHH/prefix.log`
- **日志时区配置** — 新增 `Timezone` 配置项（IANA 时区名），默认 UTC，跨时区部署行为一致
- **Discord Bot 修复** — `Start()` ctx 已取消时立即返回，修复 flaky test

## v2.0.0 新特性

- **WAF 中间件** — Web 应用防火墙，防护 SQL 注入/XSS/路径遍历/命令注入（[middleware/waf](./middleware/waf/)）
- **API 版本化** — 路径/Header 双模式版本路由（[middleware/version](./middleware/version/)）
- **优雅降级** — 5xx/panic 自动 fallback（[middleware/fallback](./middleware/fallback/)）
- **负载卸载** — 并发/队列深度/延迟阈值卸载（[middleware/loadshed](./middleware/loadshed/)）
- **JWT 非对称签名** — 新增 RS256/ES256/EdDSA 支持（[auth/jwt](./auth/jwt/)）
- **OpenTelemetry Metrics** — Prometheus + OTel 双指标后端（[observability/metrics](./observability/metrics/)）
- **调试面板** — 路由/配置/健康/指标/构建信息一览（[transport/debug](./transport/debug/)）
- **OpenAPI 3.1** — 升级至 3.1 规范，支持 Webhooks 定义（[openapi](./openapi/)）
- **存储层全面追踪** — ClickHouse/S3/MinIO/Neo4j 客户端 `EnableTracing`，自动上报 OTel span
- **消息队列追踪** — Pub/Sub 和 JobQueue 支持 trace context 跨消息传播，factory 自动包装
- **`servex dev`** — 开发模式，air 热重载（文件变更自动重启）
- **`servex gen k8s`** — K8s Deployment/Service/HPA manifest 生成

## 设计原则

- **KISS** - 保持简单，避免过度设计
- **DRY** - 抽象通用模式，减少重复代码
- **SOLID** - 单一职责，接口隔离
- **可组合** - 中间件可自由组合，支持 Endpoint / HTTP / gRPC 三层
- **可扩展** - 所有核心组件基于接口，支持自定义实现

## 工厂函数命名规范

| 模式 | 说明 | 示例 |
| --- | --- | --- |
| `NewXXX` | 返回 `(T, error)` | `rdbms.NewDatabase(cfg, log)` |
| `MustNewXXX` | 失败时 panic | `rdbms.MustNewDatabase(cfg, log)` |
| `DefaultConfig` | 返回默认配置 | `logger.DefaultConfig()` |

## 中间件执行顺序

推荐顺序（从外到内）：WAF → Logging → Tracing → Metrics → RateLimit → CircuitBreaker → Retry → Timeout → Recovery

详见各中间件包的 README。

## License

MIT License
