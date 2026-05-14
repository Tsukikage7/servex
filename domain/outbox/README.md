# outbox

`github.com/Tsukikage7/servex/v2/outbox` -- Outbox 模式（事务性消息发送）。

## 概述

outbox 包实现事务发件箱模式（Transactional Outbox Pattern），将消息与业务数据在同一数据库事务中持久化，由异步 Relay 轮询投递到消息队列，保证业务操作与消息发送的最终一致性。

## 功能特性

- 事务安全：消息与业务数据在同一事务中写入，避免数据不一致
- 异步投递：Relay 后台轮询 pending 消息并发送到消息队列
- 自动重试：发送失败自动标记，超时消息自动重置为 pending 状态
- 自动清理：定期清理已发送的历史消息
- 基于 GORM 实现，支持 MySQL、PostgreSQL、SQLite
- 行级锁优化：支持 SELECT FOR UPDATE SKIP LOCKED（SQLite 自动降级）

## 消息状态流转

```
Pending(0) --FetchPending--> Processing(1) --发送成功--> Sent(2)
                                  |
                                  +--发送失败--> Failed(3)
                                                    |
                                          ResetStale --> Pending(0)
```

## API

### 类型

| 类型 | 说明 |
|------|------|
| `OutboxMessage` | 发件箱消息模型，包含 ID、Topic、Key、Value、Headers、Status、RetryCount、LastError、CreatedAt 等 |
| `MessageStatus` | 消息状态枚举 |

### MessageStatus 常量

| 常量 | 值 | 说明 |
|------|-----|------|
| `StatusPending` | `0` | 待发送 |
| `StatusProcessing` | `1` | 发送中 |
| `StatusSent` | `2` | 已发送 |
| `StatusFailed` | `3` | 发送失败 |

### 构造函数

| 函数 | 说明 |
|------|------|
| `NewOutboxMessage(msg *messaging.Message) *OutboxMessage` | 从 messaging.Message 创建 OutboxMessage |
| `HeadersToJSON(headers map[string]string) string` | 序列化 headers 为 JSON |
| `NewGORMStore(db database.Database) *GORMStore` | 从 database.Database 创建 Store |
| `NewGORMStoreFromDB(db *gorm.DB) *GORMStore` | 从 *gorm.DB 创建 Store |
| `NewRelay(store, producer, opts...) (*Relay, error)` | 创建 Relay 中继器 |

### Store 接口

| 方法 | 说明 |
|------|------|
| `SaveTx(ctx, tx, msgs...) error` | 在指定事务中保存消息 |
| `FetchPending(ctx, limit) ([]*OutboxMessage, error)` | 拉取待发送消息并标记为 Processing |
| `MarkSent(ctx, ids) error` | 批量标记为已发送 |
| `MarkFailed(ctx, id, errMsg) error` | 标记发送失败 |
| `ResetStale(ctx, staleDuration) (int64, error)` | 重置超时消息为 Pending |
| `Cleanup(ctx, before) (int64, error)` | 清理指定时间前的已发送消息 |
| `AutoMigrate() error` | 自动迁移表结构 |

### Relay 方法

| 方法 | 说明 |
|------|------|
| `Start(ctx) error` | 启动中继器（轮询 + 清理两个后台协程） |
| `Stop(ctx) error` | 优雅关闭中继器 |

### 配置选项 (Option)

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `WithLogger(logger)` | - | 设置日志记录器 |
| `WithPollInterval(duration)` | `1s` | 轮询间隔 |
| `WithBatchSize(size)` | `100` | 每次拉取的消息条数 |
| `WithMaxRetries(n)` | `3` | 最大重试次数 |
| `WithCleanupAge(duration)` | `7d` | 已发送消息保留时长 |
| `WithCleanupInterval(duration)` | `1h` | 清理任务执行间隔 |
| `WithStaleTimeout(duration)` | `5m` | Processing 状态超时阈值 |
