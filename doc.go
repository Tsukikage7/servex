// Package servex 是一个 Go 微服务开发工具包，提供构建生产级微服务所需的核心组件.
//
// # 核心
//
//   - [app] 应用生命周期管理
//   - [endpoint] Endpoint / Middleware 核心抽象
//   - [errors] 统一错误（HTTP/gRPC 状态码映射）
//   - [encoding] 编解码器接口与 HTTP 内容协商（json/proto/xml/pbjson）
//
// # 传输层
//
//   - [transport/httpserver] HTTP 服务器
//   - [transport/grpcserver] gRPC 服务器
//   - [transport/httpclient] HTTP 客户端（retry/circuitbreaker/tracing/metrics）
//   - [transport/grpcclient] gRPC 客户端（服务发现/重试/熔断/追踪/负载均衡）
//   - [transport/gateway] API 网关（gRPC+HTTP 双协议）
//   - [transport/ginserver] Gin 适配
//   - [transport/echoserver] Echo 适配
//   - [transport/hertzserver] Hertz 适配
//   - [transport/websocket] WebSocket 服务端
//   - [transport/sse] Server-Sent Events 服务端
//   - [transport/health] 健康检查（K8s 探针）
//   - [transport/graphql] GraphQL 服务器适配
//   - [transport/tls] TLS 配置工具
//   - [transport/grpcx] gRPC 工具包
//
// # 中间件
//
//   - [middleware/ratelimit] 限流（令牌桶、滑动窗口、分布式）
//   - [middleware/circuitbreaker] 熔断器
//   - [middleware/retry] 重试（指数退避）
//   - [middleware/recovery] Panic 恢复
//   - [middleware/timeout] 超时控制
//   - [middleware/cors] 跨域资源共享
//   - [middleware/idempotency] 幂等性保证
//   - [middleware/semaphore] 并发控制
//   - [middleware/logging] 请求日志
//   - [middleware/secure] 安全头
//   - [middleware/csrf] CSRF 防护
//   - [middleware/bodylimit] 请求体大小限制
//   - [middleware/signature] HMAC 请求签名验证
//   - [middleware/trace] 链路追踪增强
//
// # 认证
//
//   - [auth/jwt] JWT 认证
//   - [auth/apikey] API Key 认证
//   - [auth/rbac] 基于角色的访问控制
//
// # 可观测性
//
//   - [observability/logger] 结构化日志（Zap）
//   - [observability/metrics] Prometheus 指标收集
//   - [observability/tracing] OpenTelemetry 链路追踪
//   - [observability/logshipper] 日志投递（ES/Kafka sink）
//
// # 配置与服务发现
//
//   - [config] 配置管理（多源热加载）
//   - [discovery] 服务发现（Consul、etcd）
//
// # 存储
//
//   - [storage/cache] 缓存（内存、Redis）
//   - [storage/rdbms] 关系数据库（GORM）
//   - [storage/mongodb] MongoDB 客户端
//   - [storage/elasticsearch] Elasticsearch 客户端
//   - [storage/s3] S3 兼容对象存储
//   - [storage/lock] 分布式锁
//   - [storage/sqlx] sqlx 封装
//   - [storage/migration] 数据库迁移
//   - [storage/clickhouse] ClickHouse 客户端
//   - [storage/redis] Redis 客户端
//
// # 消息
//
//   - [messaging/pubsub] 统一 Pub/Sub 抽象（Kafka/RabbitMQ/Redis）
//   - [messaging/jobqueue] 异步任务队列（延迟、优先级、重试、死信）
//
// # 领域驱动
//
//   - [domain] 聚合根、领域事件、EventBus
//   - [domain/cqrs] 命令查询职责分离
//   - [domain/saga] Saga 分布式事务
//   - [domain/outbox] 事务发件箱模式
//   - [domain/eventsourcing] 事件溯源
//
// # 通知
//
//   - [notify] 统一通知接口（邮件/短信/推送/Webhook）
//
// # HTTP 请求分析
//
//   - [httpx] 组合中间件（客户端 IP/UA/设备/机器人检测/语言/来源/活动追踪）
//
// # AI 集成
//
//   - [llm] 统一 ChatModel / EmbeddingModel 接口
//   - [llm/provider] OpenAI / Anthropic / Gemini 适配器
//   - [llm/agent] 工具调用、对话管理、Agent 框架
//   - [llm/retrieval] RAG / 向量存储 / 文档加载 / 重排序
//   - [llm/processing] 结构化输出 / 分类 / 提取 / 翻译 / Token 计数
//   - [llm/safety] 护栏 / 内容审核
//   - [llm/serving] 语义缓存 / API Key 管理 / 计费 / 代理网关
//
// # OAuth2
//
//   - [oauth2] 第三方登录（GitHub/Google/微信）
//
// # 其他
//
//   - [openapi] Code-first OpenAPI 3.0 生成
//   - [scheduler] Cron 定时任务调度
//   - [i18n] 国际化
//   - [tenant] 多租户
//   - [collections] 数据结构（Deque/LRU/TreeMap/PriorityQueue/HashSet 等）
//   - [xutil] 工具包（ptrx/strx/randx/iox/copier/syncx/sorting 等）
//   - [validation] 输入校验
//   - [testx] 测试工具包
//
// # 业务组件
//
//   - [bizx/counter] 分布式计数器
//   - [bizx/leaderboard] 排行榜
//   - [bizx/sequence] 业务序号生成
//   - [bizx/locking] 业务锁
//   - [bizx/ratelimit] 业务配额
//   - [bizx/statemachine] 状态机
//   - [bizx/pagination] 游标分页
//   - [bizx/audit] 审计日志
//   - [bizx/feature] 特性开关
//   - [bizx/retry] 异步重试
//   - [bizx/event] 进程内事件总线
//   - [bizx/captcha] 验证码管理
package servex
