# sse

`github.com/Tsukikage7/servex/v2/transport/sse`

Server-Sent Events (SSE) 服务端实现，提供标准 SSE 协议支持，包含客户端管理、事件广播和连接生命周期回调。

## 功能特性

- 标准 SSE 协议实现，支持事件类型、ID、重试间隔
- 客户端管理：注册、注销、元数据存储
- 广播和定向发送
- 可配置心跳保活
- 连接/断开回调

## API

### Server 接口

| 方法 | 说明 |
|------|------|
| `Run(ctx context.Context) error` | 启动服务器事件循环 |
| `ServeHTTP(w, r)` | HTTP 处理器，接受 SSE 连接 |
| `Broadcast(event *Event)` | 向所有客户端广播事件 |
| `BroadcastTo(clientIDs []string, event *Event)` | 向指定客户端列表广播 |
| `Send(clientID string, event *Event) error` | 向单个客户端发送事件 |
| `Clients() []Client` | 返回所有已连接客户端 |
| `Client(id string) (Client, bool)` | 按 ID 获取客户端 |
| `Count() int` | 返回已连接客户端数量 |
| `Close() error` | 关闭服务器 |
| `OnConnect(fn func(Client))` | 设置连接回调 |
| `OnDisconnect(fn func(Client))` | 设置断开回调 |

### Client 接口

| 方法 | 说明 |
|------|------|
| `ID() string` | 返回客户端唯一 ID |
| `Send(event *Event) error` | 向该客户端发送事件 |
| `Close() error` | 关闭连接 |
| `Context() context.Context` | 返回客户端上下文 |
| `Metadata() map[string]any` | 返回元数据副本 |
| `SetMetadata(key string, value any)` | 设置元数据 |

### Event 结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `ID` | `string` | 事件 ID |
| `Event` | `string` | 事件类型 |
| `Data` | `[]byte` | 事件数据 |
| `Retry` | `int` | 重试间隔（毫秒） |

### Config 结构

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `BufferSize` | `int` | `256` | 客户端事件缓冲区大小 |
| `HeartbeatInterval` | `time.Duration` | `30s` | 心跳间隔 |
| `RetryInterval` | `int` | `3000` | 客户端重连间隔（毫秒） |
| `Headers` | `map[string]string` | `{}` | 自定义响应头 |

### 构造函数

- `NewServer(config *Config) Server` -- 创建 SSE 服务器，传入 `nil` 使用默认配置。
- `DefaultConfig() *Config` -- 返回默认配置。

### 预定义错误

- `ErrClientNotFound` -- 客户端不存在
- `ErrServerClosed` -- 服务器已关闭
- `ErrConnectionClosed` -- 连接已关闭
- `ErrNotFlusher` -- ResponseWriter 不支持 Flushing
