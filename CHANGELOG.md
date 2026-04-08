# Changelog

本项目遵循 [语义化版本](https://semver.org/lang/zh-CN/) 规范.

## [Unreleased]

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

[Unreleased]: https://github.com/Tsukikage7/servex/compare/v1.5.1...HEAD
[v1.5.1]: https://github.com/Tsukikage7/servex/compare/v1.5.0...v1.5.1
[v1.5.0]: https://github.com/Tsukikage7/servex/compare/v1.4.0...v1.5.0
[v1.4.0]: https://github.com/Tsukikage7/servex/compare/v1.3.1...v1.4.0
[v1.3.1]: https://github.com/Tsukikage7/servex/compare/v1.3.0...v1.3.1
[v1.3.0]: https://github.com/Tsukikage7/servex/compare/v1.2.0...v1.3.0
[v1.2.0]: https://github.com/Tsukikage7/servex/compare/v1.1.0...v1.2.0
[v1.1.0]: https://github.com/Tsukikage7/servex/compare/v1.0.0...v1.1.0
[v1.0.0]: https://github.com/Tsukikage7/servex/releases/tag/v1.0.0
