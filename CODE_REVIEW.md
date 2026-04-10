# servex 全量 Code Review 报告

> 审查日期: 2026-04-10
> 项目: github.com/Tsukikage7/servex (Go 1.26.1 微服务工具包)
> 审查范围: 517 个非测试源文件，涵盖全部模块
> 审查工具: Claude Opus 4.6 多 Agent 并行审查

---

## 目录

- [一、总览](#一总览)
- [二、Critical 级别问题](#二critical-级别问题)
- [三、High 级别问题](#三high-级别问题)
- [四、Medium 级别问题](#四medium-级别问题)
- [五、Low 级别问题](#五low-级别问题)
- [六、跨模块共性问题](#六跨模块共性问题)
- [七、值得肯定的设计](#七值得肯定的设计)
- [八、优先修复建议](#八优先修复建议)

---

## 一、总览

### 问题统计

| 严重级别 | 数量 |
|---------|------|
| Critical | 5 |
| High | 33 |
| Medium | 99 |
| Low | 61 |
| **合计** | **198** |

### 模块质量评分

| 模块 | 评分 | Critical | High | Medium | Low | 主要风险 |
|------|------|----------|------|--------|-----|---------|
| auth/jwt | C | 2 | 2 | 1 | - | 安全漏洞: 错误信息泄露、fail-open |
| config/source/consul | C | 1 | - | 1 | - | Bug: WithContext 返回值丢弃 |
| domain/eventsourcing | B- | 1 | 1 | 1 | - | 版本号回滚不安全 |
| storage/cache | B- | 1 | 1 | 2 | 1 | double close panic |
| collections/blockingqueue | B | 1 | - | - | - | 信号量初始化风险 |
| transport/gateway | B | - | 1 | 3 | - | 启动竞态条件 |
| transport/httpclient | B | - | 1 | 2 | 1 | 库代码 panic |
| transport/graphql | B | - | 1 | 1 | - | OOM 风险 |
| transport/sse | B | - | 1 | 1 | - | goroutine 阻塞 |
| domain/outbox | B | - | 2 | 1 | 1 | 消息无限重试循环 |
| messaging/pubsub/rabbitmq | B | - | 2 | 1 | - | reconnect goroutine 泄漏 |
| messaging/jobqueue | B | - | 1 | 2 | 1 | Redis TOCTOU 竞态 |
| oauth2 | B | - | 2 | 3 | 1 | HTTP 状态码未检查 |
| bizx/audit | B | - | 1 | 1 | 1 | send on closed channel |
| xutil/crypto | B | - | 1 | 2 | - | 安全降级: 固定验证码 |
| xutil/idgen | B | - | 1 | 1 | - | 库函数 panic |
| observability/alerting | B+ | - | 1 | 2 | 1 | goroutine 泄漏 |
| observability/tracing | B+ | - | 1 | 3 | 1 | 覆盖全局 TracerProvider |
| observability/profiling | B+ | - | 1 | 2 | - | CPU profiling 全局冲突 |
| observability/metrics | B+ | - | 1 | 1 | 1 | 标签基数爆炸 |
| app | B+ | - | 1 | 2 | 3 | errCh goroutine 泄漏 |
| config/manager | B+ | - | 2 | 3 | 2 | fmt.Printf 日志、竞态 |
| scheduler | B+ | - | 1 | 2 | - | Stop/Shutdown 语义混淆 |
| cmd/servex | B+ | - | 2 | 3 | 2 | 死代码、文件句柄泄漏 |
| storage (其余) | B+ | - | 2 | 8 | 7 | 密钥 JSON 泄露 |
| tenant | B+ | - | 1 | 2 | 1 | 日志明文打印令牌 |
| auth/rbac | B+ | - | 1 | 2 | 1 | HTTP 错误暴露内部信息 |
| domain/saga | B+ | - | 1 | 2 | 1 | 补偿操作用超时 ctx |
| collections (其余) | A- | - | 1 | 4 | 1 | ToMap 类型安全 |
| encoding | A- | - | - | 3 | 2 | CodecForRequest 返回 nil |
| errors | A- | - | 1 | 3 | 2 | Error.Is 仅比较 Code |
| observability/logger | A | - | - | 2 | 2 | buffer 未回池 |
| testx | A | - | - | 2 | 2 | NopLogger 重复 |

---

## 二、Critical 级别问题

### CRIT-1: Consul watcher WithContext 返回值丢弃 -- 导致 Watch 无法中断

**文件**: `config/source/consul/consul.go:102`

```go
opts.WithContext(w.ctx)  // 返回值被丢弃！
```

Consul API 的 `QueryOptions.WithContext()` 返回新的 `*QueryOptions` 但不修改接收者。代码丢弃了返回值，导致传给 `KV().Get()` 的 opts 没有绑定 context。调用 `Stop()` 取消 ctx 无法中断 blocking query，`Manager.Close()` 将无法及时终止 consul watcher。

**修复**: `opts = opts.WithContext(w.ctx)`

---

### CRIT-2: JWT gRPC 拦截器向客户端泄露内部错误详情

**文件**: `auth/jwt/middleware.go:243,249,278,284,322,328,360,366`

```go
return nil, status.Error(codes.Unauthenticated, err.Error())
```

`err.Error()` 可能包含签名算法信息、claims 解析细节等内部信息，帮助攻击者针对性构造攻击。与 `auth/grpc.go` 中返回固定消息 `"认证失败"` 的做法不一致。

**修复**: 统一返回固定通用错误消息 `"认证失败"`，详细错误仅输出到日志。

---

### CRIT-3: JWT 令牌撤销检查的 fail-open 策略

**文件**: `auth/jwt/jwt.go:452-458`

```go
} else if revokeErr != nil && !errors.Is(revokeErr, cache.ErrNotFound) {
    // 缓存访问错误（如 Redis 宕机），跳过撤销检查，fail-open
    j.opts.logger.With(...).Warn("[JWT] 缓存撤销标记查询失败，跳过检查")
}
```

当缓存不可用时，已撤销的令牌仍可通过验证。对于用户被封禁/密码重置后撤销令牌的场景，这是不可接受的安全风险。

**修复**: 提供可配置的 fail-close/fail-open 策略选项，高安全性场景默认 fail-close。

---

### CRIT-4: memoryCache.Close() 重复调用导致 panic

**文件**: `storage/cache/memory.go:384`

```go
func (m *memoryCache) Close() error {
    close(m.closeCh)  // 第二次调用 panic: close of closed channel
}
```

**修复**: 使用 `sync.Once` 保护 `close(m.closeCh)`。

---

### CRIT-5: EventSourcing BaseAggregate.RaiseEvent 版本号回滚在 panic 场景不安全

**文件**: `domain/eventsourcing/eventsourcing.go:114-126`

```go
a.version++
event := Event{...Version: a.version...}
if err := applier(event); err != nil {
    a.version--  // panic 时 version 永久偏移
    return err
}
```

**修复**: 使用 `oldVersion := a.version` 保存旧值，失败时恢复；增加 `defer` 保护 panic 场景。

---

## 三、High 级别问题

### 安全类

| 编号 | 文件 | 行号 | 问题描述 |
|------|------|------|---------|
| H-SEC-1 | `auth/jwt/jwt.go` | 252-276 | Refresh 刷新窗口逻辑为死代码 -- `jwt.ParseWithClaims` 在令牌过期时 `parseErr != nil`，直接返回错误，刷新窗口机制永远不生效 |
| H-SEC-2 | `auth/jwt/jwt.go` | 181-194 | `GenerateWithDuration` 跳过缓存存储，导致通过此方法签发的令牌被 `validateCachedToken` 误判为已撤销 |
| H-SEC-3 | `auth/rbac/rbac.go` | 309 | HTTP 中间件暴露内部错误详情到客户端响应 |
| H-SEC-4 | `oauth2/wechat/provider.go` | 65-66 | URL 查询参数中明文传输 appSecret（微信 API 限制，需文档标注风险） |
| H-SEC-5 | `oauth2/*/provider.go` | 多处 | 所有 OAuth2 Provider 未验证 HTTP 响应状态码，非 2xx 响应可能导致误导性错误 |
| H-SEC-6 | `tenant/middleware.go` | 43 | 日志中明文打印完整租户令牌 `logger.String("token", token)` |
| H-SEC-7 | `storage/minio/minio.go` | 57-59 | AccessKey/SecretKey 的 JSON tag 非 `"-"`，序列化时泄露密钥 |
| H-SEC-8 | `storage/rdbms/database.go` | 58 | DSN (含密码) 的 JSON tag 非 `"-"`，序列化时泄露 |
| H-SEC-9 | `xutil/crypto/crypto.go` | 27-31 | `GenerateVerificationCode` 在 rand 失败时返回硬编码 `"000000"`，验证码可预测 |

### 并发/资源泄漏类

| 编号 | 文件 | 行号 | 问题描述 |
|------|------|------|---------|
| H-CON-1 | `app/app.go` | 125-157 | errCh 未关闭，监听 goroutine 永远不会退出 |
| H-CON-2 | `config/manager.go` | 195,200,207 | 库代码使用 `fmt.Printf` 输出日志 |
| H-CON-3 | `config/manager.go` | 178-218 | watchLoop 关闭逻辑依赖 watcher 的 Stop 能中断 Next（与 CRIT-1 联动） |
| H-CON-4 | `observability/alerting/alerting.go` | 565-571 | notify goroutine 使用 `context.Background()` 无超时，且 Engine Stop 后不等待完成 |
| H-CON-5 | `observability/profiling/profiling.go` | 293-303 | CPU profiling 全局操作无互斥保护，并发 Collect 冲突 |
| H-CON-6 | `observability/tracing/tracer.go` | 86-93 | 库代码中设置全局 TracerProvider，覆盖应用已有配置 |
| H-CON-7 | `messaging/pubsub/rabbitmq/publisher.go` | 62-108 | reconnectLoop 无退出通知机制，Close 后 goroutine 可能残留 |
| H-CON-8 | `messaging/pubsub/rabbitmq/publisher.go` | 165-168 | Publish 释放锁后使用的 channel 可能已被 reconnectLoop 替换关闭 |
| H-CON-9 | `transport/sse/sse.go` | 296-358 | 服务器关闭时 unregister channel 可能阻塞 ServeHTTP goroutine |
| H-CON-10 | `scheduler/cron.go` | 157-173 | Stop() 不等待 executeJob 启动的 goroutine 完成 |

### 设计/逻辑类

| 编号 | 文件 | 行号 | 问题描述 |
|------|------|------|---------|
| H-DES-1 | `errors/errors.go` | 45-51 | `Error.Is()` 仅比较 Code，不同业务域复用同一 code 时会误匹配 |
| H-DES-2 | `transport/httpclient/config.go` | 50 | `NewFromConfig` 中 TLS 失败使用 panic，库代码不应 panic |
| H-DES-3 | `transport/gateway/server.go` | 437-443 | startHTTP 中 100ms 等待检测启动失败不可靠 |
| H-DES-4 | `transport/graphql/graphql.go` | 108 | POST 请求体无大小限制，可导致 OOM |
| H-DES-5 | `observability/metrics/prometheus.go` | 214-320 | 自定义指标标签基数爆炸，注册失败静默忽略导致数据丢失 |
| H-DES-6 | `domain/outbox/gorm/store.go` | 122-131 | ResetStale 会重置已达最大重试次数的 Failed 消息，形成无限循环 |
| H-DES-7 | `domain/outbox/relay.go` | 110-138 | relay Stop 时 ctx 被取消，send 因此失败，消息被错误标记为 Failed |
| H-DES-8 | `domain/saga/saga.go` | 128-133 | 补偿操作使用同一个已超时的 ctx，补偿可能因 ctx 过期而无法执行 |
| H-DES-9 | `messaging/jobqueue/redis/store.go` | 65-95 | Dequeue 的 ZRangeByScore + ZRem 非原子操作，高并发下大量无效竞争 |
| H-DES-10 | `bizx/audit/audit.go` | 157-165 | 异步模式 `Log` 方法释放锁后发送 channel，Close 可能在此间隙关闭 channel 导致 panic |
| H-DES-11 | `bizx/retry/retry.go` | 197-204 | Scheduler.Start 使用 startOnce，但 parent ctx 取消后无法再次启动 |
| H-DES-12 | `xutil/idgen/idgen.go` | 291-318 | `Snowflake()/ULID()/NanoID()` 便捷函数失败时 panic，应提供返回 error 的版本 |
| H-DES-13 | `collections/treemap/tree_map.go` | 261-268 | `ToMap` 返回 `map[any]V` 而非 `map[K]V`，丢失类型信息 |
| H-DES-14 | `storage/cache/redis.go` | 8 | cache 包使用过时 go-redis/v8，与 storage/redis 的 v9 版本不一致 |
| H-DES-15 | `cmd/servex/proto.go` | 553-575 | findGoModule 中 `defer f.Close()` 在 for 循环内，累积未关闭文件句柄 |
| H-DES-16 | `cmd/servex/new.go` | 182-208 | renderTemplate 模板执行失败留下不完整的空文件 |
| H-DES-17 | `observability/logshipper/shipper.go` | 127-129 | dropOnFull=false 且 Shipper 已 Close 时，Ship 调用方 goroutine 永久阻塞 |

---

## 四、Medium 级别问题

### 4.1 核心模块 (app / config / encoding / errors)

| 编号 | 文件 | 行号 | 问题描述 |
|------|------|------|---------|
| M-CORE-1 | `app/app.go` | 149 | 100ms 硬编码检测启动失败，应可配置 |
| M-CORE-2 | `app/app.go` | 179-230 | shutdown 总返回 nil，调用者无法感知关闭失败 |
| M-CORE-3 | `config/manager.go` | 129-141 | Close 读取 watchers 未加锁，与 Watch 并发时数据竞争 |
| M-CORE-4 | `config/manager.go` | 213-216 | 观察者回调同步执行无 panic recovery |
| M-CORE-5 | `config/manager.go` | 222-242 | 多 KV 覆盖语义对 slice 字段不准确（json.Unmarshal 追加而非覆盖） |
| M-CORE-6 | `config/source/consul/consul.go` | 98-128 | 初始 WaitIndex=0 导致首次虚假配置变更通知 |
| M-CORE-7 | `config/source/k8s/k8s.go` | 230-250 | 每次 Next 重建 k8s watch 连接，频繁 TCP 建立/关闭 |
| M-CORE-8 | `config/source/k8s/k8s.go` | 245 | 仅处理 Modified 事件，忽略 Deleted，配置删除后继续使用旧值 |
| M-CORE-9 | `encoding/codec.go` | 74 | CodecForRequest 在 JSON codec 未注册时返回 nil |
| M-CORE-10 | `encoding/pbjson/http.go` | 31-39 | defer Body.Close 在 ReadAll 之后，错误路径 body 未关闭 |
| M-CORE-11 | `encoding/pbjson/http.go` | 32 | ReadAll 无大小限制，可 OOM |
| M-CORE-12 | `errors/http.go` | 42 | WriteError 忽略 json.Encode 错误 |
| M-CORE-13 | `errors/http.go` | 62-68 | HTTPErrorHandler 是空操作中间件，名称误导 |
| M-CORE-14 | `errors/grpc.go` | 63,67 | Metadata nil map 传递给 ErrorInfo |

### 4.2 认证/授权/多租户

| 编号 | 文件 | 行号 | 问题描述 |
|------|------|------|---------|
| M-AUTH-1 | `auth/jwt/jwt.go` | 414-419 | 仅验证模式下 signingKey() 返回 nil，Generate 产生不明确错误 |
| M-AUTH-2 | `auth/middleware.go` | 66 | Endpoint 中间件授权时 action/resource 传空字符串 |
| M-AUTH-3 | `auth/grpc.go` | 176-229 | gRPC Bearer Token 提取逻辑冗余不一致 |
| M-AUTH-4 | `auth/rbac/rbac.go` | 193 | 使用标准库 log 而非项目统一 logger |
| M-AUTH-5 | `auth/rbac/rbac.go` | 202-220 | 角色继承链深度无上限，可能导致大量 DB 查询 |
| M-AUTH-6 | `auth/apikey/authenticator.go` | 66-79 | ConstantTimeCompare 在长度不等时立即返回，非常量时间 |
| M-AUTH-7 | `oauth2/*/provider.go` | 多处 | UserInfo/Refresh 未限制响应体大小（wechat 已正确限制） |
| M-AUTH-8 | `oauth2/*/provider.go` | 51 | PKCE verifier 内存存储不支持多实例部署 |
| M-AUTH-9 | `oauth2/oauth2.go` | 41 | Provider 接口 Exchange 不接受 state 参数，PKCE 受限 |
| M-AUTH-10 | `oauth2/*` | 多处 | 三个 Provider 大量重复代码（PKCE、getString、cleanupLoop） |
| M-AUTH-11 | `tenant/gorm/scope.go` | 46-55 | AutoInject 无租户时静默跳过，可能导致数据泄漏到全局 |
| M-AUTH-12 | `tenant/gorm/scope.go` | 19-31 | Scope 无租户时不过滤，查询返回所有租户数据 |

### 4.3 传输层

| 编号 | 文件 | 行号 | 问题描述 |
|------|------|------|---------|
| M-TRANS-1 | `transport/httpserver/codec.go` | 45 | 请求体大小限制硬编码 10MB 不可配置 |
| M-TRANS-2 | `transport/grpcserver/server.go` | 87-135 | gRPC Server Start 缺少重复启动保护 |
| M-TRANS-3 | `transport/gateway/server.go` | 290 | startGRPC 中 Serve 错误完全被忽略 |
| M-TRANS-4 | `transport/gateway/server.go` | 159-190 | Stop 方法错误覆盖，应用 errors.Join |
| M-TRANS-5 | `transport/gateway/options.go` | 639-649 | buildMethodSkipper 死代码 |
| M-TRANS-6 | `transport/httpclient/client.go` | 多处 | New 与 NewSimple 中间件顺序不一致 |
| M-TRANS-7 | `transport/httpclient/middleware.go` | 126-147 | CircuitBreaker 5xx 路径语义混乱 |
| M-TRANS-8 | `transport/websocket/gorilla.go` | 30 | gorillaClient 使用 context.Background() 而非请求上下文 |
| M-TRANS-9 | `transport/websocket/gorilla.go` | 192-198 | 默认 CheckOrigin 允许所有来源且每次请求打印警告 |
| M-TRANS-10 | `transport/tls/tls.go` | 79,108,140 | 使用标准库 log.Println 而非结构化日志 |
| M-TRANS-11 | `transport/graphql/graphql.go` | 196 | Playground endpoint 在 JS 上下文中 HTML 转义不充分 |
| M-TRANS-12 | `transport/debug/debug.go` | 多处 | Debug 面板无默认访问控制，暴露敏感信息 |
| M-TRANS-13 | `middleware/cors/cors.go` | 128-129 | Credentials+Wildcard 冲突组合无初始化警告 |
| M-TRANS-14 | `middleware/timeout/http.go` | 34-82 | 超时后 handler goroutine 仍可写入 ResponseWriter |
| M-TRANS-15 | `transport/sse/sse.go` | 62-88 | Event.Bytes() 未处理多行数据，不符合 SSE 规范 |
| M-TRANS-16 | `observability/metrics/middleware.go` | 43-58 | responseWriter 未实现 http.Flusher/Hijacker |

### 4.4 存储层

| 编号 | 文件 | 行号 | 问题描述 |
|------|------|------|---------|
| M-STOR-1 | `storage/cache/redis.go` | 244 | Lua 脚本返回值 type assertion 无保护，可 panic |
| M-STOR-2 | `storage/cache/memory.go` | 174-191 | Exists 方法 TOCTOU 竞态条件 |
| M-STOR-3 | `storage/clickhouse/client.go` | 46-57 | Ping 失败时未关闭已打开的连接 |
| M-STOR-4 | `storage/elasticsearch/client.go` | 58 | Ping 超时硬编码 5 秒 |
| M-STOR-5 | `storage/lock/lock.go` | 82-89 | WithLock 中 Unlock 错误被静默忽略 |
| M-STOR-6 | `storage/migration/migration.go` | 28-50 | Registry 非并发安全 |
| M-STOR-7 | `storage/migration/runner.go` | 多处 | 每次公开方法调用都执行 AutoMigrate |
| M-STOR-8 | `storage/minio/minio.go` | 124-161 | NewClient 不验证连接可达性 |
| M-STOR-9 | `storage/mongodb/mongodb.go` | 60-61 | SocketTimeout 配置定义但从未使用 |
| M-STOR-10 | `storage/neo4j/neo4j.go` | 269 | Run 方法硬编码 AccessModeRead，无法执行写操作 |
| M-STOR-11 | `storage/rdbms/encrypt_column.go` | 31 | EncryptColumn.Key 导出字段明文暴露 |
| M-STOR-12 | `storage/redis/redis.go` | 196-211 | NewClient 不验证连接可达性 |
| M-STOR-13 | `storage/s3/client.go` | 448-488 | 分片上传 Complete 失败时未 Abort，孤儿分片产生存储费用 |
| M-STOR-14 | `storage/s3/client.go` | 45-51 | 使用已废弃的 EndpointResolverWithOptions API |

### 4.5 领域驱动/消息

| 编号 | 文件 | 行号 | 问题描述 |
|------|------|------|---------|
| M-DDD-1 | `domain/event.go` | 18-19 | BaseEvent 硬编码 time.Now()，测试不可控 |
| M-DDD-2 | `domain/eventbus.go` | 45-48 | EventBus.Publish 任一 handler 失败即中断后续 |
| M-DDD-3 | `domain/async_eventbus.go` | 64-72 | Dispatch 部分事件发布成功后失败，状态不一致 |
| M-DDD-4 | `domain/cqrs/middleware/metrics.go` | 19-23 | Prometheus 指标 ConstLabels 冲突，注册失败静默忽略 |
| M-DDD-5 | `domain/saga/saga.go` | 118-126 | Saga 状态保存失败被静默忽略 |
| M-DDD-6 | `domain/saga/kvstore/store.go` | 167-171 | List 方法始终返回 nil, nil |
| M-DDD-7 | `domain/outbox/relay.go` | 175 | cleanupLoop 使用 relay ctx 而非 Background |
| M-DDD-8 | `messaging/eventbus/eventbus.go` | 214-216 | PublishAsync channel 满时阻塞持 RLock，Close 需 Lock 可死锁 |
| M-DDD-9 | `messaging/pubsub/rabbitmq/subscriber.go` | 85-142 | reconnect 后未重新注册已有消费者 |
| M-DDD-10 | `messaging/pubsub/kafka/config.go` | 11-32 | NewPublisher/SubscriberFromConfig 创建的 Client 在 Close 时不关闭 |
| M-DDD-11 | `messaging/jobqueue/redis/store.go` | 126-129 | MarkDone 忽略 Del 错误 |
| M-DDD-12 | `messaging/jobqueue/database/store.go` | 60-83 | Dequeue 乐观锁高并发效率低 |
| M-DDD-13 | `messaging/jobqueue/worker.go` | 94-96 | MarkRunning 失败后仍继续执行任务，可能重复执行 |
| M-DDD-14 | `scheduler/cron.go` | 361-363 | runWithRetry 中 time.Sleep 不响应 context 取消 |
| M-DDD-15 | `scheduler/cron.go` | 259 | executeJob 使用 Background()，Shutdown 无法取消运行中任务 |
| M-DDD-16 | `discovery/consul.go` | 322-324 | Discover 硬编码使用 GRPC tags 过滤，无法发现 HTTP 服务 |
| M-DDD-17 | `discovery/etcd.go` + `nacos.go` | 88, 111 | RegisterWithHealthEndpoint 忽略 healthEndpoint 参数 |
| M-DDD-18 | `notify/dispatcher.go` | 114-127 | Close 持 RLock 遍历并关闭 sender，与 Send 并发不安全 |
| M-DDD-19 | `notify/webhook/dispatcher.go` | 34-69 | Webhook 投递无重试机制 |

### 4.6 业务扩展/集合/工具

| 编号 | 文件 | 行号 | 问题描述 |
|------|------|------|---------|
| M-BIZ-1 | `bizx/event/event.go` | 163-168 | Publish 异步任务的 ctx 可能已被取消 |
| M-BIZ-2 | `bizx/counter/counter.go` | 263-264 | Redis MGet 类型断言可能 panic |
| M-BIZ-3 | `bizx/captcha/captcha.go` | 148-155 | Verify 先 IncrAttempts 再 Get，即使验证码不存在也消耗次数 |
| M-BIZ-4 | `bizx/retry/retry.go` | 232-248 | sem 阻塞时 ctx 取消导致 wg 永远等待 |
| M-BIZ-5 | `bizx/locking/locking.go` | 208-243 | 可重入锁 token 为空时行为不直观 |
| M-BIZ-6 | `bizx/workflow/workflow.go` | 312-344 | executeParallel 名不副实，未真正并行 |
| M-BIZ-7 | `bizx/workflow/workflow.go` | 229-309 | Execute 中间状态未持久化，崩溃恢复可能重复执行 |
| M-BIZ-8 | `bizx/statemachine/statemachine.go` | 77-128 | Guard/Action 回调在持有写锁时执行 |
| M-BIZ-9 | `bizx/ratelimit/ratelimit.go` | 86-93 | time.Truncate 对 24h 窗口受时区影响 |
| M-BIZ-10 | `collections/priorityqueue/priority_queue.go` | 69-70 | Pop 未将移除位置清零，GC 无法回收 |
| M-BIZ-11 | `collections/priorityqueue/priority_queue.go` | 106-113 | ToSlice 是破坏性操作，命名未体现 |
| M-BIZ-12 | `collections/multimap/multi_map.go` | 32 | Get 返回内部切片引用，可被外部修改 |
| M-BIZ-13 | `collections/multimap/multi_map.go` | 55-58 | RemoveValue 修改原始切片影响外部持有者 |
| M-BIZ-14 | `collections/lrucache/lru_cache.go` | 96-116 | GetOrPut 持锁调用 loader，长时间阻塞缓存 |
| M-BIZ-15 | `collections/deque/deque.go` | 196-215 | Rotate 逐个 Pop/Push 可能触发多次 resize |
| M-BIZ-16 | `xutil/crypto/crypto.go` | 58-73 | GenerateBusinessID/64 同样在 rand 失败时返回固定值 |
| M-BIZ-17 | `xutil/crypto/crypto.go` | 46-55 | GenerateRandomInt64 当 max=MaxInt64 时溢出 |
| M-BIZ-18 | `xutil/copier/copier.go` | 106-161 | 切片/map 仅浅拷贝，文档未说明 |
| M-BIZ-19 | `xutil/syncx/cond.go` | 45-63 | Signal 在 unsubscribe 之后发出可能丢失唤醒 |
| M-BIZ-20 | `xutil/syncx/segment_keys_lock.go` | 42-53 | hash 取模分布不均，建议强制 size 为 2 的幂 |
| M-BIZ-21 | `xutil/iox/iox.go` | 82-96 | LimitReadCloser 达限返回自定义 error 而非 io.EOF |

### 4.7 可观测性/测试/CLI

| 编号 | 文件 | 行号 | 问题描述 |
|------|------|------|---------|
| M-OBS-1 | `observability/alerting/alerting.go` | 391-430 | evaluateAll 持写锁时调用 provider.Query（网络 IO），阻塞读操作 |
| M-OBS-2 | `observability/alerting/alerting.go` | 204 | New 不验证 provider 是否为 nil |
| M-OBS-3 | `observability/logger/encoder_console.go` | 29 | buffer.Buffer 从 pool Get 后未 Put 回收 |
| M-OBS-4 | `observability/logger/zap.go` | 58-61 | Console 格式强制开启 caller，覆盖用户配置 |
| M-OBS-5 | `observability/logshipper/elasticsearch.go` | 57 | 每条日志生成 UUID，高吞吐场景性能瓶颈 |
| M-OBS-6 | `observability/profiling/profiling.go` | 275-276 | Stop 重置全局 runtime 设置影响其他代码 |
| M-OBS-7 | `observability/profiling/profiling.go` | 376-397 | collectAll 串行采集，CPU profiling 阻塞其他类型 |
| M-OBS-8 | `observability/slo/slo.go` | 313-346 | Collect 持 RLock 时获取 ot.mu.Lock |
| M-OBS-9 | `observability/slo/slo.go` | 222-263 | SLO Window 语义未实现，使用累计值而非滑动窗口 |
| M-OBS-10 | `observability/tracing/tracer.go` | 48 | 默认强制 WithInsecure (HTTP)，应可配置 |
| M-OBS-11 | `observability/tracing/grpc.go` | 320 | 使用字符串比较 `err.Error() == "EOF"` 而非 errors.Is |
| M-OBS-12 | `observability/tracing/grpc.go` | 163 | StreamServerInterceptor 中 SendHeader 可能失败 |
| M-OBS-13 | `testx/mock.go` | 13-31 | NopLogger 与 logger.Nop() 功能重复 |
| M-OBS-14 | `testx/container.go` | 35-40 | Container.Close 忽略 context 参数 |
| M-OBS-15 | `cmd/servex/new.go` | 28-68 | runNew/runGen/runProto 等死代码函数 |
| M-OBS-16 | `cmd/servex/wizard.go` | 224-226 | 向导函数直接修改包级全局变量 |
| M-OBS-17 | `cmd/servex/dev.go` | 57-88 | devRunner.stop 可能 double Wait cmd |
| M-OBS-18 | `cmd/servex/main.go` | 396-400 | protoBreakingAgainst 用字符串比较判断是否用户设置 |

---

## 五、Low 级别问题

> 共 61 项，以下按模块分组列出。

### 核心模块 (7 项)

| 文件 | 行号 | 描述 |
|------|------|------|
| `doc.go` | 96-104 | LLM 文档与实际状态不一致 |
| `app/app.go` | 194 | Go 1.22+ WaitGroup.Go 最低版本要求 |
| `app/app.go` | 54-59 | Use 方法对 nil server 无防御 |
| `app/options.go` | 33-38 | defaultOptions 不初始化 hooks |
| `config/loader.go` | 21 | os.IsNotExist 应改用 errors.Is |
| `config/loader.go` | 99-100 | LoadWithSearch 丢弃原始错误信息 |
| `encoding/pbjson/json.go` | 9-18 | 全局可变的 MarshalOptions/UnmarshalOptions |

### 认证/授权/多租户 (6 项)

| 文件 | 行号 | 描述 |
|------|------|------|
| `auth/http.go` | 185-196 | HTTPSkipPaths 不处理尾随斜杠 |
| `auth/rbac/rbac.go` | 111-127 | 缓存选项定义但未使用 |
| `oauth2/state/memory.go` | 61-64 | MemoryStore.Close 不幂等 |
| `tenant/scope.go` | 21-33 | WhereClause column 参数未做注入防护 |
| `auth/http.go` + `tenant/http.go` | 177/116 | writeHTTPError 重复定义 |
| `encoding/proto/proto.go` | 1-2 | 包名 proto 但实际输出 JSON 格式 |

### 传输层 (8 项)

| 文件 | 行号 | 描述 |
|------|------|------|
| `transport/httpserver/server.go` | 134 | errAlreadyStarted 未使用 |
| `transport/httpserver/codec.go` | 47 | LimitReader 截断数据静默 |
| `transport/httpclient/response.go` | 多处 | Body 多次关闭风险 |
| `transport/websocket/middleware.go` | 50-100 | RateLimitMiddleware 请求路径上做清理 |
| `transport/health/checker.go` | 176-195 | CompositeChecker 串行执行 |
| `middleware/csrf/csrf.go` | 62 | HttpOnly 默认 true 与 Double Submit 矛盾 |
| `openapi/schema.go` | 多处 | 缺少递归类型保护 |
| `validation/validation.go` | 198 | ParseErrors 用直接类型断言而非 errors.As |

### 存储层 (8 项)

| 文件 | 行号 | 描述 |
|------|------|------|
| `storage/cache/memory.go` | 342-353 | MGet 在 RLock 下不更新 lastAccessed |
| `storage/clickhouse/client.go` | 59 | 日志语言不统一（中英混用） |
| `storage/lock/redis.go` | 27 | held map 无定期清理，可能内存泄漏 |
| `storage/migration/runner.go` | 235-242 | findMigration 每次 copy + sort |
| `storage/minio/minio.go` | 174-179 | GetObject 返回值需调用方 Close，文档未说明 |
| `storage/mongodb/mongodb.go` | 52 | URI (含密码) JSON tag 非 "-" |
| `storage/neo4j/neo4j.go` | 251,260,271 | Session.Close 错误被 nolint 忽略无日志 |
| `storage/s3/client.go` | 237-244 | CopyObject 未做空 key 校验 |

### 领域/消息 (6 项)

| 文件 | 行号 | 描述 |
|------|------|------|
| `domain/async_eventbus.go` | 84-86 | JSONEventConverter 序列化可能丢失非导出字段 |
| `domain/cqrs/command.go` | 15-21 | commandHandlerFunc 已定义但未使用 |
| `domain/outbox/outbox.go` | 94-96 | Headers JSON 反序列化错误被忽略 |
| `domain/saga/saga.go` | 85-87 | Build() 空 steps 时 panic 而非返回 error |
| `messaging/pubsub/redis/publisher.go` | 92-95 | Close 不关闭底层 Redis 连接 |
| `messaging/jobqueue/kafka/store.go` | 62-76 | Kafka Store Dequeue/Mark 均为空操作 |

### 业务/集合/工具 (10 项)

| 文件 | 行号 | 描述 |
|------|------|------|
| `bizx/audit/audit.go` | 106-107 | bufferSize 修正修改局部变量而非 opts |
| `bizx/event/event.go` | 157-160 | 每次 Publish 按优先级排序 |
| `bizx/counter/counter.go` | 114-126 | 内存实现 GetWindow 不清理过期记录 |
| `bizx/captcha/captcha.go` | 307-309 | Redis Delete 未删除 cooldown key |
| `bizx/retry/retry.go` | 288-290 | 退避延迟 math.Pow 可能溢出 |
| `bizx/locking/locking.go` | 155-159 | time.After 在循环中不停止旧 timer |
| `bizx/sequence/sequence.go` | 93-98 | storeKey 每次调用 time.Now 可能跨日期 |
| `collections/slicesx/slices.go` | 350-368 | Take/Skip 返回子切片而非副本 |
| `collections/delayqueue/delay_queue.go` | 33 | capacity 参数暴露但未使用 |
| `xutil/templatex/funcs.go` | 43 | 使用已废弃的 strings.Title |

### 可观测性/测试/CLI (8 项)

| 文件 | 行号 | 描述 |
|------|------|------|
| `observability/alerting/alerting.go` | 528 | `_ = rule` 无意义赋值 |
| `observability/logger/zap.go` | 318-322 | Close 吞没所有 Sync 错误（含文件写入） |
| `observability/logger/config.go` | 54-76 | Validate 不做大小写标准化 |
| `observability/slo/slo.go` | 119 | registry 字段未使用 |
| `observability/tracing/middleware.go` | 89-97 | responseWriter 未追踪响应大小 |
| `testx/mock.go` | 81-90 | Fatal/Panic 不终止测试 |
| `cmd/servex/new.go` | 78 | 模板名 main.go.tmpl 输出为 README.md |
| `cmd/servex/infra.go` | 709-732 | init() 中删除 componentRegistry 条目 |

---

## 六、跨模块共性问题

### 6.1 敏感信息 JSON 序列化泄露不一致

**正确处理 (`json:"-"`)**: clickhouse, elasticsearch, neo4j, redis, s3, cache

**存在泄露风险**: minio (AccessKey/SecretKey), rdbms (DSN), mongodb (URI)

**建议**: 统一所有含密码/密钥字段为 `json:"-"`。

### 6.2 日志语言和框架不统一

| 模块 | 日志方式 | 语言 |
|------|---------|------|
| config/manager | fmt.Printf | 中文 |
| auth/rbac | 标准库 log | 中文 |
| transport/tls | 标准库 log | 英文 |
| transport/websocket | 标准库 log | 英文 |
| storage/cache | logger.Logger | 中文 |
| storage/clickhouse | logger.Logger | 英文 |

**建议**: 统一使用 `observability/logger.Logger`，语言统一为中文。

### 6.3 连接验证行为不一致

| 创建时验证 | 不验证 |
|-----------|-------|
| cache/redis, clickhouse, elasticsearch, mongodb, neo4j | redis, minio, s3 |

**建议**: 提供统一的 Lazy/Eager 连接策略选项。

### 6.4 多处 goroutine 缺乏 panic recovery

涉及: messaging/eventbus worker, rabbitmq reconnectLoop, etcd keepAlive, alerting notify 等。

**建议**: 所有后台 goroutine 添加 `defer func() { if r := recover(); r != nil { ... } }()`。

### 6.5 错误消息混用中英文

- `domain/errors.go`: "未找到"、"并发冲突"（中文）
- `eventsourcing/errors.go`: "eventsourcing: event store is nil"（英文）
- `outbox/errors.go`: "outbox: Store 为空"（混合）

**建议**: 统一错误消息语言。

---

## 七、值得肯定的设计

### 架构层面
1. **三层中间件体系** (Endpoint / HTTP / gRPC) 设计优雅，职责分离清晰
2. **泛型应用得当**: `Manager[T]`、`CommandHandler[C,R]`、`AggregateRoot[ID]` 等类型安全且使用自然
3. **统一 Options 模式**: 所有模块遵循 `Option func(*options)` + `defaultOptions()` + `applyOptions()` 三件套
4. **Config + Validate + ApplyDefaults 三件套**: 存储层全部模块风格一致

### 安全层面
5. **JWT "none" 算法防护**: Validate 和 Refresh 都明确校验签名算法
6. **密钥长度校验**: HMAC 密钥强制至少 32 字节
7. **常量时间比较**: apikey/StaticValidator 和 jwt/whitelist 均使用 `subtle.ConstantTimeCompare`
8. **PKCE 支持**: GitHub/Google provider 实现了 S256
9. **State 一次性消费**: Memory 和 Redis 实现确保 CSRF state 不可重放
10. **租户缓存 key hash**: 使用 SHA-256 避免缓存中存储原始令牌
11. **SQL 注入防护**: sorting 包的 `isSafeFieldName` 白名单校验

### 健壮性
12. **Outbox FetchPending**: 使用 `SELECT FOR UPDATE SKIP LOCKED`
13. **事件溯源快照容错**: Load 在快照失败时 fallback 到从头重放
14. **Scheduler CAS 单例**: `atomic.Int32` + `CompareAndSwap` 零锁开销
15. **编译期接口合规检查**: 多处 `var _ Interface = (*Impl)(nil)`
16. **深拷贝防御**: MemoryStore 实现普遍使用深拷贝防止外部修改内部状态
17. **GC 友好**: deque PopFront/PopBack 将移除元素置零
18. **防死循环**: workflow 引擎 maxSteps 限制、事件总线 closeOnce

---

## 八、优先修复建议

### P0 -- 立即修复 (影响安全性和正确性)

| 编号 | 问题 | 预估工作量 |
|------|------|-----------|
| CRIT-1 | Consul WithContext 返回值丢弃 | 1 行 |
| CRIT-2 | JWT gRPC 错误信息泄露 | 8 处改动 |
| CRIT-3 | JWT fail-open 策略 | 新增配置项 |
| CRIT-4 | memoryCache Close double panic | sync.Once |
| H-SEC-6 | 日志明文打印租户令牌 | 1 行 |
| H-SEC-7/8 | MinIO/RDBMS 密钥 JSON 泄露 | JSON tag 改 "-" |
| H-SEC-9 | 验证码 rand 失败返回固定值 | 改为返回 error |

### P1 -- 尽快修复 (影响稳定性)

| 编号 | 问题 | 预估工作量 |
|------|------|-----------|
| H-CON-1 | app errCh goroutine 泄漏 | 关闭 channel |
| H-CON-2 | config 库代码 fmt.Printf | 注入 logger |
| H-CON-7 | RabbitMQ reconnectLoop 泄漏 | 重构关闭协议 |
| H-DES-6 | Outbox ResetStale 无限循环 | 增加 retry_count 过滤 |
| H-DES-8 | Saga 补偿用超时 ctx | 独立 context |
| H-DES-10 | audit Log send on closed channel | 持锁发送或 select |
| CRIT-5 | EventSourcing version 回滚 | defer 保护 |

### P2 -- 计划修复 (影响质量)

| 类别 | 涉及问题 | 建议 |
|------|---------|------|
| 密钥泄露统一 | M-STOR 相关 | 全部敏感字段 json:"-" |
| 日志框架统一 | 跨模块 | 替换所有 fmt.Printf/log.Println |
| 连接验证统一 | redis, minio, s3 | 创建时 Ping 或提供 Lazy 选项 |
| 请求体大小限制 | graphql, pbjson, httpserver | 统一 LimitReader |
| goroutine panic recovery | 跨模块 | 所有后台 goroutine 加 recover |
| 错误语言统一 | 跨模块 | 统一为中文或英文 |
| 死代码清理 | cmd/servex, gateway | 删除 runNew/runGen/buildMethodSkipper 等 |

---

> 报告生成工具: Claude Opus 4.6 (7 Agent 并行审查)
> 审查耗时: ~7 分钟
