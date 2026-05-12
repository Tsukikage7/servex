# Changelog

本项目遵循 [语义化版本](https://semver.org/lang/zh-CN/) 规范.

## [Unreleased]

## [v2.1.1] - 2026-04-17

### Added
- `observability/logger`：日志文件结构改为 `prefix/YYYYMMDD/prefix.log`，小时轮转为 `prefix/YYYYMMDDHH/prefix.log`
- `observability/logger`：新增 `Timezone` 配置项和 `WithLocation` 选项，默认 UTC，支持任意 IANA 时区

### Changed
- `transport/response`：统一 response 与 errors 双错误体系，新增 `Code.ToError()`，`GRPCStatus`/拦截器委托 `errors` 包实现

### Fixed
- `transport/botserver/discord`：`Start()` 在 ctx 已取消时立即返回，修复 flaky test

## [v2.1.0] - 2026-04-16

### Added
- 新增 `llm/compose` DAG 编排引擎（Graph[I,O]、四范式节点、条件边 Branch、共享 State、Runnable[I,O]）
- 新增 `llm/compose` Eino 级别自动流式转换（Invoke 模式 auto-concat、Streaming 模式 auto-boxing）
- 新增 `llm/compose` Callback/Tracing 横切面（CallbackHandler + OTel Span 自动注入）
- 新增 `llm/agent` EventToken 真流式 token 粒度输出
- 新增 `llm/agent` FanOut 并行 Fan-out/Fan-in 多 Agent 协作
- 新增 `llm/agent` Blackboard 黑板共享状态 + BlackboardAgent
- 新增 `llm/agent` Interrupt/Resume 人工干预（CheckpointStore + InterruptPolicy + Resume）
- 新增 `llm/agent` AgentCallbackHandler 回调接口
- 新增 `llm/retrieval/vectorstore/memory` 内存向量存储
- 新增 `llm/retrieval/vectorstore/pgvector` PostgreSQL pgvector 适配器
- 新增 `llm/retrieval/vectorstore/redis` Redis Search 适配器
- 新增 `llm/retrieval/vectorstore/elasticsearch` Elasticsearch kNN 向量搜索（支持 BM25+kNN 混合搜索）
- 新增 `llm/provider/ollama` Ollama 本地模型适配器
- 新增 `llm/provider/deepseek` DeepSeek 适配器
- 新增 `llm/provider/bedrock` AWS Bedrock Converse API 适配器

## [v2.0.14] - 2026-04-15

### Changed
- 修正安装路径，补全 botserver 包和近期新特性文档

## [v2.0.13] - 2026-04-15

### Changed
- justfile 全面升级（goimports/fmt-check/air 热重载/deploy/health/vuln/changelog）

## [v2.0.12] - 2026-04-14

### Added
- `storage/neo4j` EnableTracing 实现
- `messaging/pubsub/factory` EnableTracing 自动包装 TracingPublisher
- `messaging/jobqueue/factory` 新增工厂方法 + TracingClient/TracingWorker

## [v2.0.11] - 2026-04-14

### Added
- 消息队列 trace context 传播（Kafka/RabbitMQ/Redis Pub/Sub）

## [v2.0.10] - 2026-04-14

### Added
- Tracing 全面支持（MongoDB/Redis/Elasticsearch/S3/Neo4j/Kafka/RabbitMQ）

## [v2.0.9] - 2026-04-14

### Fixed
- 统一应用生命周期日志为中文

## [v2.0.8] - 2026-04-14

### Fixed
- CLI 模板 import 统一 v2 路径（修复 42 处 v1/v2 混用）
- `discovery` 新增 `AdvertiseAddr`，解决容器内服务注册地址问题

## [v2.0.7] - 2026-04-14

### Changed
- CLI 模板和 examples 统一使用 servex errors 包装错误

## [v2.0.6] - 2026-04-14

### Changed
- transport/auth 错误统一为 servex errors 包装

## [v2.0.5] - 2026-04-14

### Added
- `health` Response 增加 Version 字段
- httpserver/grpcserver/gateway 新增 WithVersion Option

## [v2.0.4] - 2026-04-14

### Added
- `response` 新增 GatewayErrorHandler，gateway 错误响应支持 i18n

## [v2.0.3] - 2026-04-14

### Added
- CLI 模板内置 googleapis proto，支持离线 buf 编译

## [v2.0.2] - 2026-04-14

### Added
- CLI monorepo + project 模板新增 deploy/docker 目录结构

### Fixed
- 统一所有存储/传输模块日志消息为中文

## [v2.0.1] - 2026-04-13

### Added
- `botserver` 平台无关 Bot 接口、命令路由、中间件链、对话状态存储
- `botserver/telegram` Telegram Webhook Bot 实现
- `botserver/discord` Discord Gateway Bot 实现
- `botserver/bottest` Bot 测试工具包

## [v2.0.0] - 2026-04-12

### Changed
- 全面代码审查：154 项问题修复（10 Critical + 34 High + 79 Medium + 31 Low）
- 安全/并发/资源管理/错误处理/API 设计全面加固

## [v1.9.7] - 2026-04-10

### Changed
- 代码审查修复 + buf 迁移 + logger 重构 + 架构解耦

## [v1.9.6] - 2026-04-09

### Fixed
- 修复文档与代码不一致的问题

## [v1.9.5] - 2026-04-09

### Changed
- 强制 Wire 依赖注入，InfraDef 重命名为 ComponentDef

## [v1.9.4] - 2026-04-09

### Added
- CLI 新增 `add aggregate` 和 `add proto` 命令

## [v1.9.3] - 2026-04-09

### Added
- 补齐 transport skills 参考文档

## [v1.9.2] - 2026-04-08

### Added
- Example 函数 100% 覆盖所有包

## [v1.9.1] - 2026-04-08

### Added
- 为核心模块添加 Example 函数

## [v1.9.0] - 2026-04-08

### Added
- 新增电商订单系统完整示例项目（DDD + CQRS 架构）

## [v1.8.0] - 2026-04-08

### Added
- CLI 代码生成器（monorepo/project/service/gateway/aggregate/proto）
- 六边形架构（port + adapter）

## [v1.7.10] - 2026-04-08

### Added
- DDD 测试骨架生成命令

## [v1.6.16] - 2026-04-08

### Added
- 告警规则引擎

## [v1.6.15] - 2026-04-08

### Added
- 持续性能剖析支持

## [v1.6.14] - 2026-04-08

### Added
- K8s ConfigMap/Secret 配置源

## [v1.5.1] - 2026-04-08

### Fixed
- 修复 outbox Relay 测试竞态条件

### Changed
- 规范化全局 Go 注释风格

## [v1.5.0] - 2026-04-08

### Added
- 新增 `observability/logshipper` 日志投递模块（ES/Kafka sink，异步批量）
- 新增 `transport/grpcx` gRPC 工具包（流包装/Metadata/错误/健康检查）
- 新增 `middleware/trace` 链路追踪增强中间件（trace-id 传播/日志注入/下游传递）
- 配置 CI/CD 工作流

### Changed
- 重构 Claude Code Skills：保留 2 个活跃 skill，20 个子模块移至参考文档
- 提升整体测试覆盖率

## [v1.4.0] - 2026-04-05

### Added
- 新增 `bizx` 业务组件模块，包含 12 个子包：
  - `bizx/counter` 分布式计数器（精确计数/滑动窗口）
  - `bizx/leaderboard` 排行榜（Top N/排名/分页）
  - `bizx/sequence` 业务序号生成
  - `bizx/locking` 业务锁（可重入/读写锁/续期）
  - `bizx/ratelimit` 业务配额（按用户/租户限流）
  - `bizx/statemachine` 状态机（状态/事件/守卫/回调）
  - `bizx/pagination` 游标分页
  - `bizx/audit` 审计日志（操作记录/变更追踪）
  - `bizx/feature` 特性开关（灰度/百分比/白名单）
  - `bizx/retry` 异步重试（持久化/指数退避/死信）
  - `bizx/event` 进程内事件总线（通配符/优先级/异步）
  - `bizx/captcha` 验证码管理

## [v1.3.1] - 2026-04-05

### Added
- 新增 `validation` 输入校验模块（go-playground/validator 封装，中英文错误消息）
- 新增 `auth/rbac` 基于角色的访问控制
- 新增 `xutil/idgen` ID 生成器
- 新增 `middleware/signature` HMAC 请求签名验证中间件
- 新增 `storage/redis` Redis 客户端（完整数据类型 + Pipeline + Pub/Sub）

## [v1.3.0] - 2026-04-05

### Added
- 新增 `middleware/secure` 安全头中间件（HSTS/CSP/X-Frame-Options）
- 新增 `middleware/csrf` CSRF 防护中间件
- 新增 `middleware/bodylimit` 请求体大小限制中间件
- 新增 `transport/tls` TLS 配置工具（证书/mTLS/版本控制）

## [v1.2.0] - 2026-04-05

### Changed
- 重构 `ai/` 模块为 `llm/`，统一命名空间

### Added
- 新增 `llm/agent` 自主 Agent 框架（ReAct/PlanExecute/Supervisor/Pipeline）
- 新增 `llm/retrieval/rag` RAG 管线
- 新增 `llm/serving/cache` 语义缓存
- 新增 `llm/safety/guardrail` 输入输出护栏
- 新增 `llm/serving/proxy` AI API 代理网关
- 新增 `llm/agent/chain` 多步 LLM 编排
- 新增 `llm/agent/memory` 持久化记忆
- 新增 `llm/retrieval/rerank` 重排序器
- 新增 `llm/retrieval/document` 文档加载器
- 新增 `llm/eval` LLM 输出评估
- 新增 `llm/processing/tokenizer` Token 计数器
- 新增 `llm/safety/moderation` 内容审核
- 新增 `llm/serving/apikey` API Key 管理
- 新增 `llm/serving/billing` 用量计费
- 新增 `llm/processing/classifier` 文本分类器
- 新增 `llm/processing/extractor` 信息提取
- 新增 `llm/processing/translator` 翻译器

## [v1.1.0] - 2026-04-04

### Added
- 新增 `testx` 测试工具包（NopLogger/TestLogger/Container/HTTPTest/Fixture）
- 新增 `storage/migration` 数据库迁移（Go DSL）
- 新增 `storage/clickhouse` ClickHouse 客户端
- 新增 `transport/graphql` GraphQL 服务器适配
- 新增 `domain/eventsourcing` 事件溯源

## [v1.0.0] - 2026-04-04

### Added
- 项目初始骨架：`endpoint`、`errors`、`encoding`
- `xutil` 工具包与 `collections` 数据结构
- `observability/logger` 结构化日志（Zap）
- `observability/metrics` Prometheus 指标收集
- `observability/tracing` OpenTelemetry 链路追踪
- `config` 配置管理（多源热加载）与 `discovery` 服务发现
- 中间件：限流、熔断、重试、恢复、超时、CORS、请求 ID、幂等、并发控制、请求日志
- 传输层：HTTP/gRPC 服务器与客户端、API 网关、Gin/Echo/Hertz 适配、WebSocket、SSE、健康检查
- `storage` 存储：缓存、RDBMS、MongoDB、Elasticsearch、S3、分布式锁、sqlx
- `auth/jwt` JWT 认证与 `auth/apikey` API Key 认证
- `tenant` 多租户与 `oauth2` 第三方登录（GitHub/Google/微信）
- `messaging/pubsub` Pub/Sub（Kafka/RabbitMQ/Redis）与 `messaging/jobqueue` 异步任务队列
- `domain` 领域驱动：聚合根、CQRS、Saga、事务发件箱
- AI 集成：OpenAI、Anthropic、Gemini 适配器，工具调用，Provider 路由
- `notify` 通知（邮件/短信/推送/Webhook）
- `openapi` Code-first OpenAPI 3.0 生成
- `scheduler` Cron 定时任务调度
- `app` 应用生命周期管理
- Claude Code Plugin 与 Skills

[Unreleased]: https://github.com/Tsukikage7/servex/compare/v2.1.1...HEAD
[v2.1.1]: https://github.com/Tsukikage7/servex/compare/v2.1.0...v2.1.1
[v2.1.0]: https://github.com/Tsukikage7/servex/compare/v2.0.14...v2.1.0
[v2.0.14]: https://github.com/Tsukikage7/servex/compare/v2.0.13...v2.0.14
[v2.0.13]: https://github.com/Tsukikage7/servex/compare/v2.0.12...v2.0.13
[v2.0.12]: https://github.com/Tsukikage7/servex/compare/v2.0.11...v2.0.12
[v2.0.11]: https://github.com/Tsukikage7/servex/compare/v2.0.10...v2.0.11
[v2.0.10]: https://github.com/Tsukikage7/servex/compare/v2.0.9...v2.0.10
[v2.0.9]: https://github.com/Tsukikage7/servex/compare/v2.0.8...v2.0.9
[v2.0.8]: https://github.com/Tsukikage7/servex/compare/v2.0.7...v2.0.8
[v2.0.7]: https://github.com/Tsukikage7/servex/compare/v2.0.6...v2.0.7
[v2.0.6]: https://github.com/Tsukikage7/servex/compare/v2.0.5...v2.0.6
[v2.0.5]: https://github.com/Tsukikage7/servex/compare/v2.0.4...v2.0.5
[v2.0.4]: https://github.com/Tsukikage7/servex/compare/v2.0.3...v2.0.4
[v2.0.3]: https://github.com/Tsukikage7/servex/compare/v2.0.2...v2.0.3
[v2.0.2]: https://github.com/Tsukikage7/servex/compare/v2.0.1...v2.0.2
[v2.0.1]: https://github.com/Tsukikage7/servex/compare/v2.0.0...v2.0.1
[v2.0.0]: https://github.com/Tsukikage7/servex/compare/v1.9.7...v2.0.0
[v1.9.7]: https://github.com/Tsukikage7/servex/compare/v1.9.6...v1.9.7
[v1.9.6]: https://github.com/Tsukikage7/servex/compare/v1.9.5...v1.9.6
[v1.9.5]: https://github.com/Tsukikage7/servex/compare/v1.9.4...v1.9.5
[v1.9.4]: https://github.com/Tsukikage7/servex/compare/v1.9.3...v1.9.4
[v1.9.3]: https://github.com/Tsukikage7/servex/compare/v1.9.2...v1.9.3
[v1.9.2]: https://github.com/Tsukikage7/servex/compare/v1.9.1...v1.9.2
[v1.9.1]: https://github.com/Tsukikage7/servex/compare/v1.9.0...v1.9.1
[v1.9.0]: https://github.com/Tsukikage7/servex/compare/v1.8.0...v1.9.0
[v1.8.0]: https://github.com/Tsukikage7/servex/compare/v1.7.10...v1.8.0
[v1.7.10]: https://github.com/Tsukikage7/servex/compare/v1.6.16...v1.7.10
[v1.6.16]: https://github.com/Tsukikage7/servex/compare/v1.6.15...v1.6.16
[v1.6.15]: https://github.com/Tsukikage7/servex/compare/v1.6.14...v1.6.15
[v1.6.14]: https://github.com/Tsukikage7/servex/compare/v1.5.1...v1.6.14
[v1.5.1]: https://github.com/Tsukikage7/servex/compare/v1.5.0...v1.5.1
[v1.5.0]: https://github.com/Tsukikage7/servex/compare/v1.4.0...v1.5.0
[v1.4.0]: https://github.com/Tsukikage7/servex/compare/v1.3.1...v1.4.0
[v1.3.1]: https://github.com/Tsukikage7/servex/compare/v1.3.0...v1.3.1
[v1.3.0]: https://github.com/Tsukikage7/servex/compare/v1.2.0...v1.3.0
[v1.2.0]: https://github.com/Tsukikage7/servex/compare/v1.1.0...v1.2.0
[v1.1.0]: https://github.com/Tsukikage7/servex/compare/v1.0.0...v1.1.0
[v1.0.0]: https://github.com/Tsukikage7/servex/releases/tag/v1.0.0
