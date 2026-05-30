// Package servex 是面向 AI 应用生产化接入的 Go 微服务工具包.
//
// servex 的核心职责是把模型访问、AI 网关、配置、传输层、认证、可观测性、
// 存储和消息基础设施纳入一套可组合的 Go 服务边界。复杂 Agent、Graph、
// RAG、长期记忆和评测能力由业务层或 adapter 显式接入，不在根模块内复刻运行时。
//
// # 稳定核心
//
//   - [app] 应用生命周期管理
//   - [endpoint] Endpoint / Middleware 核心抽象
//   - [errors] 统一错误HTTP/gRPC 状态码映射
//   - [encoding] 编解码器接口与 HTTP 内容协商json/proto/xml/pbjson
//   - [transport/httpserver] HTTP 服务器
//   - [transport/grpcserver] gRPC 服务器
//   - [transport/httpclient] HTTP 客户端retry/circuitbreaker/tracing/metrics
//   - [transport/grpcclient] gRPC 客户端服务发现/重试/熔断/追踪/负载均衡
//   - [transport/gateway] API 网关gRPC+HTTP 双协议
//   - [transport/health] 健康检查K8s 探针
//   - [middleware/ratelimit] 限流令牌桶、滑动窗口、分布式
//   - [middleware/circuitbreaker] 熔断器
//   - [middleware/retry] 重试指数退避
//   - [middleware/recovery] Panic 恢复
//   - [middleware/timeout] 超时控制
//   - [middleware/cors] 跨域资源共享
//   - [middleware/logging] 请求日志
//   - [auth/jwt] JWT 认证
//   - [auth/apikey] API Key 认证
//   - [auth/rbac] 基于角色的访问控制
//   - [observability/logger] 结构化日志Zap
//   - [observability/metrics] Prometheus 指标收集
//   - [observability/tracing] OpenTelemetry 链路追踪
//   - [config] 配置管理多源热加载
//   - [discovery] 服务发现Consul、etcd
//   - [storage/cache] 缓存内存、Redis
//   - [storage/rdbms] 关系数据库GORM
//   - [messaging/pubsub] 统一 Pub/Sub 抽象Kafka/RabbitMQ/Redis
//   - [messaging/jobqueue] 异步任务队列延迟、优先级、重试、死信
//   - [domain] 聚合根、领域事件、EventBus
//   - [domain/cqrs] 命令查询职责分离
//   - [domain/saga] Saga 分布式事务
//   - [domain/outbox] 事务发件箱模式
//   - [domain/eventsourcing] 事件溯源
//   - [llm] 统一 ChatModel / EmbeddingModel 接口
//   - [llm/provider] OpenAI / Anthropic / Gemini 适配器
//   - [llm/adapter/eino] 独立 module，CloudWeGo Eino 双向适配
//   - [llm/adapter/adk] 独立 module，Google ADK Agent / LLMAgent / Runner 适配
//   - [llm/gateway] ServeX AI 网关
//   - [llm/mcp] MCP 工具注册、策略和 llm.Tool 转换边界
//   - [llm/observability] OpenTelemetry GenAI 属性和用量记录辅助
//
// # 扩展包
//
// 适配层和重依赖实现位于子包中按需导入，例如 storage/cache/redis、
// storage/rdbms、messaging/pubsub/kafka、messaging/jobqueue/redis、
// discovery/consul、auth/jwt/grpcx、llm/adapter/eino。
//
// # 非核心包
//
// xutil、collections 和 bizx 是历史扩展能力，不再作为 servex 核心 API 叙事的一部分。
// 新代码应优先依赖稳定核心包；需要通用工具或业务组件时，应在应用层显式选择。
package servex
