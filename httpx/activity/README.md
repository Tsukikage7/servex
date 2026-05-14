# activity

`github.com/Tsukikage7/servex/v2/httpx/activity` -- 用户活动追踪。

## 概述

activity 包提供用户活跃时间追踪功能，通过消息队列异步记录用户活动事件，使用 Redis 存储在线状态与最后活跃信息。支持 HTTP 中间件与 gRPC 拦截器自动采集。

## 功能特性

- 异步解耦：通过 Kafka 等消息队列异步记录，不阻塞主业务
- Redis 存储：在线状态与最后活跃时间实时查询
- 幂等去重：同一时间窗口内多次请求只记录一次
- 采样控制：支持按比例采样，适应高流量场景
- HTTP/gRPC 中间件：自动提取用户ID与平台信息
- 批量消费：支持批量消息处理，按用户聚合只保留最新事件

## API

### 类型

| 类型 | 说明 |
|------|------|
| `Event` | 活跃事件，包含 UserID、Timestamp、EventType、Platform、DeviceID、IP、Path、Extra |
| `Status` | 用户状态，包含 UserID、IsOnline、LastActiveAt、LastPlatform、LastIP、OnlineDuration |
| `EventType` | 事件类型枚举 |
| `Tracker` | 活跃追踪器 |

### EventType 常量

| 常量 | 值 | 说明 |
|------|-----|------|
| `EventTypeRequest` | `"request"` | 普通请求 |
| `EventTypeHeartbeat` | `"heartbeat"` | 心跳 |
| `EventTypeLogin` | `"login"` | 登录 |
| `EventTypeLogout` | `"logout"` | 登出 |
| `EventTypePageView` | `"pageview"` | 页面浏览 |

### 接口

| 接口 | 说明 |
|------|------|
| `Producer` | 消息生产者，定义 `Publish(ctx, topic, event)` 方法 |
| `Store` | 存储接口，定义 SetLastActive、GetLastActive、SetOnline、IsOnline、GetOnlineCount 等方法 |

### Tracker 方法

| 方法 | 说明 |
|------|------|
| `NewTracker(opts ...Option) *Tracker` | 创建追踪器 |
| `Track(ctx, event) error` | 记录活跃事件 |
| `GetStatus(ctx, userID) (*Status, error)` | 获取用户活跃状态 |
| `GetMultiStatus(ctx, userIDs) (map[string]*Status, error)` | 批量获取状态 |
| `IsOnline(ctx, userID) bool` | 检查用户是否在线 |
| `GetOnlineCount(ctx) (int64, error)` | 获取在线用户数 |

### 工具函数

| 函数 | 说明 |
|------|------|
| `MarshalEvent(event) ([]byte, error)` | 序列化事件 |
| `UnmarshalEvent(data) (*Event, error)` | 反序列化事件 |
| `NewRedisStore(client, opts...) *RedisStore` | 创建 Redis 存储 |
| `NewKafkaProducer(producer) *KafkaProducer` | 创建 Kafka 生产者 |
| `NewConsumer(tracker) *Consumer` | 创建消费者 |

### 配置选项

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `WithStore(store)` | - | 设置存储后端 |
| `WithProducer(producer)` | - | 设置消息生产者 |
| `WithUserIDExtractor(fn)` | 从 auth.Principal 提取 | 自定义用户ID提取 |
| `WithTopic(topic)` | `"user_activity_events"` | Kafka topic |
| `WithAsyncMode(bool)` | `true` | 异步模式 |
| `WithOnlineTTL(duration)` | `5m` | 在线状态 TTL |
| `WithDedupeWindow(duration)` | `30s` | 去重窗口 |
| `WithSampleRate(rate)` | `1.0` | 采样率 |
