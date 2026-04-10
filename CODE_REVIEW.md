# servex 全量代码审查报告

> 审查日期: 2026-04-10
> 审查范围: 507 个源文件, ~71K 行 Go 代码 (不含测试和示例)
> 审查工具: 人工逐文件审查 + 静态分析

---

## 目录

- [一、总览](#一总览)
- [二、问题统计](#二问题统计)
- [三、严重问题 (Critical)](#三严重问题-critical)
- [四、高优先级问题 (High)](#四高优先级问题-high)
- [五、中等优先级问题 (Medium)](#五中等优先级问题-medium)
- [六、低优先级问题 (Low)](#六低优先级问题-low)
- [七、做得好的方面](#七做得好的方面)
- [八、优先修复建议](#八优先修复建议)

---

## 一、总览

| 模块组 | 源文件数 | 严重 | 高 | 中 | 低 | 小计 |
|--------|---------|------|------|------|------|------|
| 核心基础设施 (app, config, discovery, endpoint, errors, encoding, transport) | ~80 | 2 | 6 | 19 | 8 | 35 |
| 认证与安全 (auth, oauth2, middleware, tenant) | ~60 | 1 | 8 | 16 | 7 | 32 |
| 存储与领域 (storage, domain, messaging) | ~70 | 2 | 9 | 18 | 1 | 30 |
| 业务扩展与工具 (bizx, xutil, collections, httpx, notify, i18n, etc.) | ~90 | 4 | 6 | 10 | 10 | 30 |
| 可观测性与LLM (observability, llm, cmd/servex) | ~80 | 1 | 5 | 16 | 5 | 27 |
| **合计** | **~380** | **10** | **34** | **79** | **31** | **154** |

---

## 二、问题统计

### 按严重程度

| 严重程度 | 数量 | 占比 |
|---------|------|------|
| 严重 (Critical) | 10 | 6.5% |
| 高 (High) | 34 | 22.1% |
| 中 (Medium) | 79 | 51.3% |
| 低 (Low) | 31 | 20.1% |

### 按问题类型

| 类型 | 数量 |
|------|------|
| 安全性问题 | 42 |
| 并发安全 | 32 |
| 错误处理 | 28 |
| 资源管理 | 26 |
| API 设计 | 12 |
| 性能问题 | 8 |
| 数据一致性 | 6 |

---

## 三、严重问题 (Critical)

### C-01. [安全] RabbitMQ Publisher 并发发布确认错乱 -- 消息可能丢失

- **文件**: `messaging/pubsub/rabbitmq/publisher.go:106-145`
- **模块**: messaging/pubsub/rabbitmq
- **描述**: `Publish` 方法获取 `p.mu.Lock()` 拿到 `ch` 和 `confirms` 后立即 `p.mu.Unlock()`，然后逐条发布并从 `confirms` channel 读取确认。多个 goroutine 同时调用 `Publish` 时共用同一个 `confirms` channel，goroutine A 发布的消息确认可能被 goroutine B 读取，导致 B 误认为自己的消息已确认，而 A 超时等待。
- **修复建议**: 将整个发布+确认过程放在 mutex 保护内，或使用 delivery tag 进行精确匹配。

### C-02. [安全] crypto 包验证码生成使用非安全随机数

- **文件**: `xutil/crypto/crypto.go:24-42`
- **模块**: xutil/crypto
- **描述**: `GenerateVerificationCode`、`GenerateRandomInt32`、`GenerateRandomInt64` 等函数位于 `crypto` 包中，但使用 `math/rand/v2` 而非 `crypto/rand`。包名暗示加密安全性，调用者会以为产出密码学安全的随机数。同文件的 `GenerateID` 已正确使用 `crypto/rand`，存在安全性不一致。
- **修复建议**: 统一使用 `crypto/rand`，或将非安全函数移到 `randx` 包。

### C-03. [并发] 可重入锁未真正基于 goroutine 身份实现

- **文件**: `bizx/locking/locking.go:172-258`
- **模块**: bizx/locking
- **描述**: `reentrantLock` 声称是可重入锁，但没有任何 goroutine 身份识别机制。`owner` 字段存在但未用于判断调用者身份。任何 goroutine 在 `count > 0` 时都能"重入"成功，完全违背可重入锁语义。goroutine A 持有锁时，goroutine B 调用 `Lock()` 会直接增加计数并返回成功。
- **修复建议**: 通过 `runtime.Stack` 提取 goroutine ID 或使用 context 传递锁令牌。如不能可靠实现，应在文档中声明这是引用计数锁而非可重入锁。

### C-04. [安全] 验证码比对存在时序攻击风险

- **文件**: `bizx/captcha/captcha.go:162`
- **模块**: bizx/captcha
- **描述**: `stored != code` 使用普通字符串比较。虽然验证码较短(6位)且有最大尝试次数限制，实际可利用性较低，但作为安全库应使用常量时间比较。
- **修复建议**: 使用 `subtle.ConstantTimeCompare([]byte(stored), []byte(code))`。

### C-05. [安全] 微信 OAuth redirect_uri 使用了 appID (功能性 bug)

- **文件**: `oauth2/wechat/provider.go:49`
- **模块**: oauth2/wechat
- **描述**: `AuthURL` 中 `redirect_uri` 被设置为 `p.opts.appID`，这是明显的 bug。应使用回调 URL，但 options 结构中甚至没有 `redirectURL` 字段。这会导致 OAuth 回调失败或授权码发送到错误地址。
- **修复建议**: 在 `options` 中添加 `redirectURL` 字段并正确使用。

### C-06. [并发] memoryCache.Close() 重复调用导致 panic

- **文件**: `storage/cache/memory.go:384-388`
- **模块**: storage/cache
- **描述**: `Close()` 直接调用 `close(m.closeCh)`，无任何防护。若被调用两次（如 defer 和显式调用），对已关闭 channel 执行 close 触发 panic。
- **修复建议**: 使用 `sync.Once` 保护 close 操作。

### C-07. [并发] 服务器启动后的错误被静默丢弃

- **文件**: `app/app.go:110-140`
- **模块**: app
- **描述**: `start()` 中 errCh 仅在 100ms 内检查第一个错误。100ms 后启动失败的服务器错误写入 errCh 后再也不会被读取，无任何日志记录。
- **修复建议**: 启动后台 goroutine 持续消费 errCh，对错误记录日志并触发 shutdown。

### C-08. [安全] 服务 ID 使用不安全随机数 + 可能碰撞

- **文件**: `discovery/factory.go:50-65`
- **模块**: discovery
- **描述**: `GenerateServiceID` 使用 `math/rand` 配合 `time.Now().UnixNano()` 作为种子。`randomNum` 范围仅 0-999999，同一秒内同一服务有碰撞风险。显式创建 `rand.New(rand.NewSource(...))` 在同一纳秒内可能产生相同 ID。
- **修复建议**: 使用 UUID (项目已依赖 `github.com/google/uuid`)。

### C-09. [安全] Gemini API Key 暴露在 URL 参数中

- **文件**: `llm/provider/gemini/gemini.go:166,207,253`
- **模块**: llm/provider/gemini
- **描述**: Gemini 客户端将 `apiKey` 拼接到 URL 查询参数 `?key=<apiKey>` 中，会出现在 HTTP 中间件日志、代理日志、错误消息中。流式请求返回非 200 时错误响应体可能包含完整 URL (含 Key)。
- **修复建议**: 在 logging middleware 中对 URL 参数脱敏；提供安全风险文档说明。

### C-10. [安全] LLM 护栏绕过 -- 仅检查 Content 不检查 Parts

- **文件**: `llm/safety/guardrail/guardrail.go:42-53,68-85`
- **模块**: llm/safety/guardrail
- **描述**: `MaxLength`、`KeywordFilter`、`PIIDetector` 只检查 `m.Content` 字段，不检查 `m.Parts` 中的多模态文本内容。攻击者可以将有害内容放在 `Parts` 的 `ContentTypeText` 片段中绕过所有护栏。
- **修复建议**: 遍历 `Parts` 中的文本内容并一并检查。

---

## 四、高优先级问题 (High)

### H-01. [并发] WebSocket RateLimitMiddleware 使用非并发安全的 map

- **文件**: `transport/websocket/middleware.go:48-80`
- **描述**: `limits` map 在闭包中被多个 goroutine 并发访问，无锁保护，确定的 data race。且断开连接的客户端 entry 永远不会被清理，导致内存泄漏。
- **修复建议**: 使用 `sync.Map` 或 `sync.Mutex` 保护，并在客户端断开时清理。

### H-02. [安全] errors.WriteErrorFrom 向客户端暴露内部错误详情

- **文件**: `errors/http.go:44-56`
- **描述**: 当 `err` 不是 `*Error` 类型时，`err.Error()` 可能包含数据库连接字符串、文件路径等敏感内容，直接暴露给客户端。
- **修复建议**: 非 `*Error` 类型使用通用消息 "内部服务器错误"，原始错误仅记录到日志。

### H-03. [资源] httpserver DecodeCodecRequest 未限制请求体大小

- **文件**: `transport/httpserver/codec.go:44`
- **描述**: `io.ReadAll(r.Body)` 无大小限制。恶意客户端可发送超大请求体导致 OOM。
- **修复建议**: 使用 `io.LimitReader` 或 `http.MaxBytesReader`。

### H-04. [资源] etcd keepalive goroutine 泄漏

- **文件**: `discovery/etcd.go:140-143`
- **描述**: keepalive 消费 goroutine 在 `Unregister` 未调用时永远泄漏。多次 `Register` 会累积多个此类 goroutine。
- **修复建议**: 为每个 lease 维护 cancel context，在 `Unregister` 和 `Close` 时取消。

### H-05. [安全] GraphQL playground XSS 风险

- **文件**: `transport/graphql/graphql.go:177-207`
- **描述**: `playgroundHTML` 将 `endpoint` 参数直接拼接进 HTML/JavaScript 中，若包含单引号等特殊字符将导致 XSS 注入。
- **修复建议**: 对 `endpoint` 进行 JavaScript 字符串转义，或使用 `html/template`。

### H-06. [资源] Gateway startHTTP 中启动错误被 default 分支丢弃

- **文件**: `transport/gateway/server.go:414-435`
- **描述**: `select` 语句包含 `default` 分支，HTTP 服务器启动错误几乎总会被静默丢弃。
- **修复建议**: 移除 `default` 分支，改用短暂定时器等待。

### H-07. [错误] httpclient/grpcclient New() 中 panic 替代返回错误

- **文件**: `transport/httpclient/client.go:47-55`, `transport/grpcclient/client.go`
- **描述**: `New` 函数签名已返回 `(*Client, error)`，但验证失败时使用 panic。库代码不应 panic。
- **修复建议**: 将 panic 替换为 `return nil, errors.New(...)`。

### H-08. [安全] MongoDB maskURI 未实现 -- 密码明文记录到日志

- **文件**: `storage/mongodb/client.go:67-70`
- **描述**: `maskURI` 注释说应遮盖密码，但直接返回原始 URI。MongoDB URI 含明文密码会输出到日志。
- **修复建议**: 使用 `net/url.Parse` 解析 URI，密码部分替换为 `***`。

### H-09. [安全] Neo4j 连接日志明文输出含密码的 URI

- **文件**: `storage/neo4j/neo4j.go:205`
- **描述**: 日志中直接记录完整 URI，包含认证信息。
- **修复建议**: 只记录 host 和 database，或脱敏处理。

### H-10. [并发] EventBus.Publish 中 slice append 竞态

- **文件**: `domain/eventbus.go:37`
- **描述**: `append(b.handlers[event.EventName()], b.handlers["*"]...)` 在 RLock 下执行 append，可能复用底层数组导致 data race。
- **修复建议**: 在 RUnlock 后手动合并到新 slice。

### H-11. [一致性] Outbox Relay 将无法投递的消息标记为已发送

- **文件**: `domain/outbox/relay.go:148-154`
- **描述**: 达到最大重试次数时 `send()` 返回 nil (成功)，消息被标记为 `StatusSent`。导致消息丢失。
- **修复建议**: 返回 `ErrMaxRetriesExceeded`，标记为 `StatusDead`。

### H-12. [一致性] isConcurrencyError 检测过于宽泛

- **文件**: `domain/eventsourcing/gorm/store.go:68-77`
- **描述**: 通过字符串匹配 "unique"/"duplicate"/"constraint" 检测并发冲突，可能误判非版本冲突的约束违反。
- **修复建议**: 检查具体索引名或使用数据库驱动错误码 (MySQL 1062, PostgreSQL 23505)。

### H-13. [资源] jobqueue Worker 所有状态转换错误被忽略

- **文件**: `messaging/jobqueue/worker.go:88,95,110,119`
- **描述**: `MarkRunning`/`MarkFailed`/`MarkDead`/`MarkDone` 的错误全被忽略。`MarkDone` 失败意味着已成功执行的任务可能被重复处理。
- **修复建议**: 至少记录错误日志，关键状态转换实现重试。

### H-14. [资源] Kafka/RabbitMQ/Redis Subscriber 多次 Subscribe 覆盖 cancel

- **文件**: `messaging/pubsub/kafka/subscriber.go:65-67`
- **描述**: 每次 `Subscribe` 创建新 cancel 覆盖旧值，导致首次订阅的 goroutine 无法被取消。
- **修复建议**: 维护 cancel 列表，或使用单一父 context。

### H-15. [可靠性] RabbitMQ 无连接重连机制

- **文件**: `messaging/pubsub/rabbitmq/publisher.go`, `subscriber.go`
- **描述**: 单一 AMQP Connection/Channel，断开后所有操作失败，无自动重连。
- **修复建议**: 监听 `conn.NotifyClose`，实现重连逻辑。

### H-16. [安全] JWT secretKey 无强度校验

- **文件**: `auth/jwt/jwt.go:107-109`
- **描述**: 只检查非空，不检查密钥长度。HMAC-SHA256 推荐至少 32 字节，用户可能传入 "secret" 这样的弱密钥。
- **修复建议**: 添加密钥长度校验，至少 32 字节。

### H-17. [安全] JWT 刷新时未验证签名方法

- **文件**: `auth/jwt/jwt.go:222-261`
- **描述**: `RefreshWithClaims` 第二次解析过期令牌时，keyFunc 中未检查签名方法，攻击者理论上可构造 "none" 算法令牌绕过签名验证。
- **修复建议**: 在第二次解析的 keyFunc 中也加上 HMAC 签名方法检查。

### H-18. [安全] JWT 缓存验证失败时误判为 Token 已撤销

- **文件**: `auth/jwt/jwt.go:386-393`
- **描述**: 缓存网络错误 (如 Redis 超时) 时直接返回 `ErrTokenRevoked`，导致缓存临时不可用时所有合法令牌被拒绝。
- **修复建议**: 区分 "不存在" 和 "访问错误"，访问错误时 fail-open 或返回具体错误。

### H-19. [安全] HTTP 凭据提取器默认通过 URL 查询参数传递 token

- **文件**: `auth/http.go:117-124`
- **描述**: `DefaultHTTPCredentialsExtractor` 支持从 `?access_token=` 提取令牌，URL 会出现在日志、浏览器历史、Referer 中。
- **修复建议**: 移除默认行为或作为需显式启用的选项。

### H-20. [安全] IP 限流可被 X-Forwarded-For 头伪造绕过

- **文件**: `middleware/ratelimit/http.go:59-72`
- **描述**: `IPKeyFunc` 优先信任 `X-Forwarded-For` 和 `X-Real-IP` 头，客户端可伪造不同值绕过限流。
- **修复建议**: 提供 `TrustedProxyIPKeyFunc(trustedProxies)` 变体，文档说明仅在可信代理后使用。

### H-21. [并发] 熔断器 HTTP 中间件无法正确计算失败

- **文件**: `middleware/circuitbreaker/http.go:9-21`
- **描述**: 回调始终返回 nil，熔断器无法根据请求结果判断失败，永远不会触发开路。且 handler 已写入响应后再写 503 导致 superfluous WriteHeader。
- **修复建议**: 使用 `responseRecorder` 捕获状态码，根据状态码判断成功/失败。

### H-22. [并发] timeout 模块自定义 mutex 有初始化竞态

- **文件**: `middleware/timeout/http.go:93-105`
- **描述**: 自定义 `mutex` 在 `Lock()` 中延迟初始化 channel，两个 goroutine 同时调用可能各自创建 channel，互斥失效。
- **修复建议**: 直接使用 `sync.Mutex`。

### H-23. [并发] idempotency SetNX 存在 TOCTOU 竞态

- **文件**: `middleware/idempotency/store.go:78-93`
- **描述**: 先 `Exists` 再 `SetNX`，两步间存在窗口，可能导致幂等性失效、请求重复执行。
- **修复建议**: 移除 `Exists` 检查，直接依赖 `SetNX` 原子性。

### H-24. [安全] OAuth PKCE verifier 内存泄漏

- **文件**: `oauth2/github/provider.go:39,81-82`
- **描述**: PKCE code_verifier 存储在内存 map，未完成的 OAuth 流程永远不会清理，导致 OOM。
- **修复建议**: 添加过期机制或存储到 StateStore。

### H-25. [并发] Prometheus 自定义指标动态注册 label 不一致会 panic

- **文件**: `observability/metrics/prometheus.go:213-244`
- **描述**: DCL 模式中 `extractLabels` 在锁外计算。同名指标用不同 label 集合首次调用时，后续调用在 `WithLabelValues` 时 panic。
- **修复建议**: 将 `extractLabels` 调用移到锁内。

### H-26. [并发] SummaryMemory 持有锁时同步调用 LLM (长时间阻塞)

- **文件**: `llm/agent/memory/memory.go:376-385,410-446`
- **描述**: `SummaryMemory.Add` 在持有锁时调用 LLM Generate，可能阻塞数秒至数十秒。
- **修复建议**: 将摘要操作改为异步或在摘要期间释放锁。

### H-27. [安全] LLM Proxy handler 请求体未限制大小

- **文件**: `llm/serving/proxy/handler.go:96`
- **描述**: 直接 `json.NewDecoder(r.Body).Decode()` 无大小限制，可被 DoS 攻击。
- **修复建议**: 使用 `http.MaxBytesReader`。

### H-28. [资源] 三个 LLM Provider 的 io.ReadAll 未限制读取大小

- **文件**: `llm/provider/openai/openai.go:470`, `anthropic/anthropic.go:354`, `gemini/gemini.go:449`
- **描述**: 异常大的响应 (恶意服务或 bug) 会导致 OOM。
- **修复建议**: 使用 `io.LimitReader(resp.Body, maxBodySize)`。

### H-29. [并发] bizx/locking 读写锁存在严重逻辑缺陷

- **文件**: `bizx/locking/locking.go:289-315`
- **描述**: `RLock` 通过获取/释放写锁检测写锁状态，释放后到增加 readers 间存在竞态。`readers` 字段是进程本地的，分布式场景下完全失效。
- **修复建议**: 分布式读写锁需要在分布式存储中维护读者计数。

### H-30. [错误] bizx/retry 退避忽略用户配置的 initialDelay 和 backoffMultiplier

- **文件**: `bizx/retry/retry.go:271-274`
- **描述**: `processTask` 硬编码 `time.Minute` 和 `2.0`，忽略用户通过 `Submit` 传入的选项。
- **修复建议**: 将退避参数持久化到 `Task` 结构体。

### H-31. [错误] bizx/event 异步事件缓冲区满时静默丢弃

- **文件**: `bizx/event/event.go:162-165`
- **描述**: 异步通道满时事件被静默丢弃，`Publish` 对异步 handler 不返回错误。
- **修复建议**: 默认 `errorHandler` 记录日志；考虑让 `Publish` 返回错误。

### H-32. [安全] notify/webhook/store/gorm SQL 通配符注入

- **文件**: `notify/webhook/store/gorm/store.go:71`
- **描述**: `LIKE` 查询直接拼接 `eventType`，含 `%`/`_` 时导致非预期匹配。
- **修复建议**: 对 `eventType` 进行 LIKE 转义或改用精确匹配。

### H-33. [并发] i18n Localizer 直接持有 Bundle 内部 map 引用

- **文件**: `i18n/i18n.go:124-128`
- **描述**: `Localizer.Translate` 读取 `messages` 时无锁保护，若 Bundle 并发修改嵌套 map 内容会产生 data race。
- **修复建议**: 创建 Localizer 时深拷贝消息，或持有 Bundle 引用并在读取时加读锁。

### H-34. [并发] memoryCache.Client() 暴露内部 map 引用

- **文件**: `storage/cache/memory.go:391-393`
- **描述**: `Client()` 直接返回内部 `m.data` map，调用方可绕过锁保护直接读写。
- **修复建议**: 返回 nil 或浅拷贝。

---

## 五、中等优先级问题 (Medium)

### 核心基础设施

| ID | 文件 | 问题 |
|----|------|------|
| M-01 | `app/app.go:61-88` | `Run()` 在 `start()` 失败后 `running` 标记未清理，再次调用永远返回 `ErrRunning` |
| M-02 | `app/app.go:39-40` | `New()` 在 logger 为 nil 时 panic，库代码应返回错误 |
| M-03 | `config/manager.go:90-104` | `Watch()` 对 `m.watchers` append 无锁保护，多 goroutine 调用产生竞态 |
| M-04 | `config/manager.go:155-168` | `watchLoop` 静默吞没配置热加载的解码和验证错误 |
| M-05 | `discovery/consul.go:324` | `Discover` 硬编码 `"grpc"` 标签，HTTP 服务永远无法被发现 |
| M-06 | `discovery/nacos.go:170-203` | `Unregister` 接收 ctx 参数但从未使用 |
| M-07 | `transport/httpserver/server.go:87-125` | pprof 端点在 `authFn == nil` 时无认证保护 |
| M-08 | `transport/httpserver/server.go:128-159` | `Start` 中服务器启动的微妙竞态条件 |
| M-09 | `transport/websocket/gorilla.go:84-86` | `SetContext` 无锁保护，`readPump`/`writePump` 并发读取产生 data race |
| M-10 | `transport/websocket/gorilla.go:189-193` | WebSocket Upgrader 默认允许所有 Origin，易受 CSRF 攻击 |
| M-11 | `transport/sse/helpers.go:120-184` | SSE Broker 所有操作对 `topics` map 无并发保护 |
| M-12 | `transport/sse/sse.go:318` | SSE ServeHTTP 中注册通道为无 select 的阻塞发送，Run() 未运行时永远阻塞 |
| M-13 | `transport/httpclient/client.go:22-24` | 服务发现缓存 TTL 硬编码 10s，无法配置 |
| M-14 | `transport/grpcclient/client.go:166-173` | 已废弃 `grpc.DialContext` 和新 `grpc.NewClient` 混合使用 |
| M-15 | `transport/tls/tls.go:132-146` | 允许配置 TLS 1.0/1.1 (已被 RFC 8996 废弃) |
| M-16 | `transport/gateway/server.go:316` | Gateway gRPC 连接硬编码使用 insecure 凭据 |
| M-17 | `transport/httpserver/config.go:58-62` | `NewFromConfig` TLS 错误导致 panic，应返回错误 |
| M-18 | `errors/grpc.go:43` | gRPC 错误序列化忽略 JSON marshal 错误 |
| M-19 | `config/source/etcd/etcd.go:45` | `Load` 使用 `context.Background()` 无法传递超时/取消 |

### 认证与安全

| ID | 文件 | 问题 |
|----|------|------|
| M-20 | `auth/middleware.go:26-69` | Principal 的 `IsExpired` 未被中间件主动检查 |
| M-21 | `auth/jwt/jwt.go:119,160` | JWT 仅支持 HMAC 对称签名，缺乏 RSA/ECDSA 支持 |
| M-22 | `auth/jwt/middleware.go:149-150` | JWT 中间件向客户端暴露内部错误信息 |
| M-23 | `auth/jwt/whitelist.go:122-130` | 白名单路径前缀匹配过于宽松，`/api/public` 匹配 `/api/publicadmin` |
| M-24 | `auth/rbac/rbac.go:305-307` | RBAC HTTP 中间件向客户端泄漏数据库错误 |
| M-25 | `auth/rbac/rbac.go:199-217` | 角色继承无深度限制，可能导致栈溢出 |
| M-26 | `oauth2/wechat/provider.go:130-141` | 微信 Provider HTTP 响应无大小限制 |
| M-27 | `oauth2/wechat/provider.go:62-63` | 微信 OAuth appSecret 在 URL 中明文传递 |
| M-28 | `oauth2/state/redis.go:61-73` | Redis StateStore Validate 非原子 (Get+Del)，state 可被重用 |
| M-29 | `middleware/csrf/csrf.go:77-153` | CSRF token 未绑定用户会话，存在 cookie tossing 风险 |
| M-30 | `middleware/cors/cors.go:62-82` | 默认 CORS 允许所有来源 `*` |
| M-31 | `middleware/signature/signature.go:157` | 签名中间件读取整个请求体，无大小限制 |
| M-32 | `middleware/signature/signature.go:94-105` | 签名不包含 HTTP 方法和路径，可跨端点重放 |
| M-33 | `middleware/ratelimit/distributed.go:91-107` | 分布式限流 Expire/IncrementBy 非原子 |
| M-34 | `middleware/timeout/endpoint.go:48-51` | 超时中间件 goroutine 可能泄漏 |
| M-35 | `middleware/gzip/gzip.go:85-88` | gzip Writer 归还 pool 前未 Reset |
| M-36 | `middleware/adaptive/adaptive.go:303-317` | CPU 负载估算方式不准确 (goroutine 数 / CPU 数) |
| M-37 | `tenant/cache.go:93` | CachedResolver 缓存 key 直接使用 token 值 |
| M-38 | `middleware/secure/secure.go:75-77` | 签名中间件不包含 HTTP 方法和路径 |

### 存储与领域

| ID | 文件 | 问题 |
|----|------|------|
| M-39 | `storage/cache/config.go:9` | 所有模块 Config 密码字段可被 JSON 序列化到日志 (普遍问题) |
| M-40 | `storage/lock/redis.go:115-140` | 分布式锁 timer Reset 在首次循环有竞态风险 |
| M-41 | `storage/rdbms/encrypt_column.go:44-61` | 加密密钥长度未在构造时校验 |
| M-42 | `storage/redis/redis.go:188-194` | NewClient 先 Validate 后 ApplyDefaults，顺序颠倒 |
| M-43 | `storage/s3/client.go:105-113` | `BucketExists`/`ObjectExists` 吞掉网络错误等真实错误 |
| M-44 | `domain/eventsourcing/repository.go:69-78` | 快照保存错误被静默忽略 |
| M-45 | `domain/eventsourcing/repository.go:98-107` | 快照加载失败导致聚合加载完全失败 (应 fallback) |
| M-46 | `domain/outbox/relay.go:140-145` | ResetStale 可能干扰当前批次正在处理的消息 |
| M-47 | `domain/saga/saga.go:110-111` | Saga.Execute 互斥锁导致同一 Saga 定义无法并发执行 |
| M-48 | `domain/saga/saga.go:98-100` | defaultIDGenerator 使用 UnixNano，并发时可能碰撞 |
| M-49 | `domain/saga/saga.go:121-129` | Saga 状态保存失败被降级为 warn 但继续执行 |
| M-50 | `messaging/pubsub/kafka/subscriber.go:136-157` | ConsumeClaim 向 channel 发送可能永久阻塞 |
| M-51 | `messaging/pubsub/redis/subscriber.go:103-110` | readGroup/readStream 错误被静默忽略，无退避 |
| M-52 | `messaging/jobqueue/worker.go:133-136` | Worker.Close() 不阻止新任务处理 (`closed` 是死代码) |
| M-53 | `messaging/eventbus/eventbus.go:204-212` | PublishAsync channel 满时阻塞，Close 后发送会 panic |
| M-54 | `messaging/pubsub/factory/factory.go:27-33` | 工厂配置中敏感凭据可被序列化 |

### 业务扩展与工具

| ID | 文件 | 问题 |
|----|------|------|
| M-55 | `bizx/audit/audit.go:88-119` | 异步 Logger 无优雅关闭机制，goroutine 泄漏 |
| M-56 | `bizx/audit/audit.go:116` | processAsync 使用 `context.Background()` 丢失原始 context |
| M-57 | `bizx/workflow/workflow.go:215-290` | Execute 无死循环保护，环形引用导致无限循环 |
| M-58 | `bizx/workflow/workflow.go:467-476` | copyData 仅做浅拷贝，嵌套引用共享底层数据 |
| M-59 | `collections/delayqueue/delay_queue.go:28-29` | Delay() 比较器返回动态值，违反堆不变式 |
| M-60 | `xutil/templatex/templatex.go:100-117` | RenderString 使用 text/template，不可信输入存在代码注入风险 |
| M-61 | `bizx/feature/feature.go:264` | Redis Store List 使用 KEYS 命令，阻塞主线程 |
| M-62 | `bizx/ratelimit/ratelimit.go:278-286` | Redis Reset SCAN 逐个删除，错误被静默忽略 |
| M-63 | `collections/treemap/tree_map.go:229-236` | ToMap 返回 `map[any]V` 丧失泛型类型安全 |
| M-64 | `bizx/counter/counter.go:113-125` | IncrWindow 滑动窗口内存无界增长 |

### 可观测性与LLM

| ID | 文件 | 问题 |
|----|------|------|
| M-65 | `observability/alerting/alerting.go:405-429` | evaluateAll 持有写锁时调用外部 Provider 查询 |
| M-66 | `observability/alerting/alerting.go:564` | notify 使用 `context.Background()` 的 goroutine 不会被取消 |
| M-67 | `observability/profiling/profiling.go:240-257` | Stop 不等待 loop goroutine 退出 |
| M-68 | `observability/profiling/profiling.go:222-223` | `SetBlockProfileRate(1)` 记录每次阻塞，生产开销大 |
| M-69 | `observability/slo/slo.go:310-336` | Prometheus events_total 指标始终为 0 |
| M-70 | `observability/tracing/tracer.go:47` | 硬编码 `WithInsecure()` 即使 endpoint 为 https |
| M-71 | `observability/logshipper/elasticsearch.go:56` | entryID 使用 math/rand，高并发碰撞 |
| M-72 | `llm/provider/openai/openai.go:42` | 三个 LLM Provider HTTP 客户端缺少默认超时 |
| M-73 | `llm/agent/memory/memory.go:540-574` | EntityMemory.extractEntities 注释说异步但实际同步 |
| M-74 | `llm/agent/conversation/conversation.go:54-66` | Chat 错误时 "回滚" 实际是添加空消息污染记忆 |
| M-75 | `llm/serving/proxy/handler.go:174` | Proxy 错误响应泄露内部细节给客户端 |
| M-76 | `llm/serving/billing/billing.go:246` | billingStreamReader 保存 context，生命周期可能超过原始请求 |
| M-77 | `llm/serving/cache/cache.go:99-121` | MemoryStore 线性扫描，缺少性能警告 |
| M-78 | `llm/serving/apikey/apikey.go:250-257` | UpdateQuota 非原子操作，并发时计数丢失 |
| M-79 | `observability/metrics/prometheus.go:10-30` | Collector 接口与 PrometheusCollector 方法名不匹配 |

---

## 六、低优先级问题 (Low)

| ID | 文件 | 问题 |
|----|------|------|
| L-01 | `config/loader.go:38-39` | Load 丢弃原始错误信息 |
| L-02 | `config/source/nacos/nacos.go:94-99` | Nacos watcher OnChange channel 满时丢弃事件 |
| L-03 | `transport/httpserver/codec.go:44-48` | DecodeCodecRequest defer Close 位置在 ReadAll 之后 |
| L-04 | `transport/grpcserver/options.go:119` | WithConfig 中 EnableReflection 零值问题 |
| L-05 | `transport/tls/tls.go:79-80` | InsecureSkipVerify 设置时无运行时警告 |
| L-06 | `encoding/codec.go:49-55` | CodecForRequest 每次请求解析 MIME 类型，可缓存 |
| L-07 | `transport/httpclient/balancer.go:30-43` | RandomBalancer 使用不必要的锁 (Go 1.20+) |
| L-08 | `observability/alerting/alerting.go:581` | 浮点数精确比较 (==) |
| L-09 | `observability/tracing/tracer.go:56-57` | 丢失原始错误信息，仅返回哨兵错误 |
| L-10 | `observability/logger/writer.go:218-240` | compressFile 中 gzWriter.Close() 错误被 defer 忽略 |
| L-11 | `llm/provider/openai/stream.go:195` | errStreamClosed 已定义但未使用 |
| L-12 | `llm/agent/strategy.go:73,134` | 通过字符串匹配判断错误类型 (脆弱) |
| L-13 | `auth/http.go:147-151` | writeHTTPError 缺少 JSON 转义 |
| L-14 | `auth/apikey/authenticator.go:66-75` | StaticValidator 遍历所有 key 泄漏 key 数量 |
| L-15 | `auth/rbac/rbac.go:189-191` | GetUserRoles 静默跳过不存在的角色 |
| L-16 | `middleware/csrf/csrf.go:93-97` | 安全方法对 cookie token 只检查长度不验证 hex 格式 |
| L-17 | `middleware/cors/cors.go:153-160` | Origin 验证不支持子域名通配符 |
| L-18 | `middleware/ratelimit/ratelimit.go:129-227` | SlidingWindow timestamps 切片突发流量下增长 |
| L-19 | `middleware/circuitbreaker/grpc.go:27,44` | 使用 `==` 比较 error 而非 `errors.Is` |
| L-20 | `middleware/recovery/http.go:48` | HTTP recovery 返回空 500 响应 |
| L-21 | `middleware/secure/secure.go:75-77` | HSTS 头在 HTTP 明文连接中也设置 |
| L-22 | `tenant/http.go:115-119` | writeHTTPError JSON 注入问题 (同 L-13) |
| L-23 | `xutil/syncx/map.go:57-69` | LoadOrStoreFunc 并发场景下 fn 可能被多次执行 |
| L-24 | `xutil/strx/strx.go:108-114` | UnsafeToBytes 缺少更醒目的安全警告 |
| L-25 | `collections/` (多文件) | 非并发安全数据结构缺少明确的线程安全声明 |
| L-26 | `bizx/statemachine/statemachine.go:76-120` | Fire 持有写锁期间执行回调，长回调导致死锁风险 |
| L-27 | `xutil/sorting/gorm.go:14` | GORMScope 字段未经白名单校验时存在 ORDER BY 注入风险 |
| L-28 | `notify/webhook/receiver.go:35` | webhook receiver 未限制请求体大小 |
| L-29 | `openapi/schema.go:109-123` | parseValidateTag 不区分 min/max 是数值约束还是长度约束 |
| L-30 | `bizx/retry/retry.go:192-193` | scheduler Start 多次调用覆盖 cancel 导致 goroutine 泄漏 |
| L-31 | `bizx/captcha/captcha.go:234-239` | memoryStore Delete 未清除 cooldowns 条目 |

---

## 七、做得好的方面

1. **Options 模式一致性**: 全项目统一使用 Options 模式 + Config 驱动工厂，API 风格一致
2. **三层中间件架构**: Endpoint / HTTP / gRPC 三层抽象清晰，复用度高
3. **泛型使用得当**: `Manager[T]`、`CommandHandler[C,R]`、`AggregateRoot[ID]` 等泛型使用合理
4. **安全意识**: CSRF 使用 `crypto/rand` + `subtle.ConstantTimeCompare`；API Key 使用常量时间比较；签名验证使用 `hmac.Equal`
5. **PKCE 支持**: OAuth2 Provider 支持 S256 PKCE
6. **测试覆盖**: 458 个测试文件，测试比例接近 1:1
7. **Example 函数**: 189 个文件的 Example 函数覆盖，文档友好
8. **错误定义**: 各模块有完整的哨兵错误定义，便于错误分类处理
9. **Context 传播**: 大部分接口正确传递 context
10. **分布式限流 fail-open**: 限流模块提供 fail-open 配置选项，兼顾可用性

---

## 八、优先修复建议

### 第一优先级 (安全/数据丢失风险，应立即修复)

1. **C-05** 微信 OAuth redirect_uri bug -- 功能完全不可用
2. **C-01** RabbitMQ 并发发布确认错乱 -- 可能导致消息丢失
3. **C-03** 可重入锁语义错误 -- 锁形同虚设
4. **H-21** 熔断器 HTTP 中间件永远不会触发 -- 形同虚设
5. **H-22** timeout 自定义 mutex 竞态 -- 并发安全核心缺陷
6. **H-11** Outbox 将失败消息标记为已发送 -- 消息丢失

### 第二优先级 (安全漏洞，应在 1-2 周内修复)

7. **C-02/C-04** crypto 包安全随机数 + 验证码时序攻击
8. **C-09/C-10** Gemini API Key 泄露 + LLM 护栏绕过
9. **H-02/H-08/H-09** 内部错误/密码明文暴露到日志/客户端
10. **H-16/H-17** JWT 弱密钥 + 刷新时签名方法未验证
11. **H-20** IP 限流头伪造绕过
12. **H-01** WebSocket 限流 map 并发竞态

### 第三优先级 (可靠性/性能，应在 1 个月内修复)

13. **H-03/H-27/H-28** 请求体/响应体大小限制 (DoS 防护)
14. **H-04/H-14** goroutine 泄漏 (etcd keepalive, subscriber cancel)
15. **H-15** RabbitMQ 重连机制
16. **H-13** jobqueue Worker 状态转换错误处理
17. **H-25/H-26** Prometheus 指标 panic + LLM 内存锁阻塞
18. **H-29/H-30** 分布式读写锁 + retry 退避配置

### 第四优先级 (代码质量，可在后续迭代中修复)

19. 中等优先级问题 (M-01 ~ M-79)
20. 低优先级问题 (L-01 ~ L-31)
