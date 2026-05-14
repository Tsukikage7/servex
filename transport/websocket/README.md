# websocket

`websocket` 包提供 WebSocket 实时通信功能，基于 `gorilla/websocket` 实现，支持连接管理、心跳检测、广播和中间件扩展。

## 功能特性

- 基于 `gorilla/websocket` 实现 WebSocket 协议
- Hub 模式集中管理所有客户端连接
- 支持广播、定向广播和点对点消息
- 内置心跳检测（Ping/Pong）和超时控制
- 中间件机制：日志、Recovery、限流、消息大小限制、认证
- 客户端支持上下文和元数据存储
- 线程安全的连接管理

## 安装

```bash
go get github.com/Tsukikage7/servex/v2/transport/websocket
```

## API

### Hub 接口

连接管理中心，负责客户端注册、消息分发：

```go
type Hub interface {
    Run(ctx context.Context) error
    Register(client Client)
    Unregister(client Client)
    Broadcast(msg *Message)
    BroadcastTo(clientIDs []string, msg *Message)
    Send(clientID string, msg *Message) error
    Clients() []Client
    Client(id string) (Client, bool)
    Count() int
    Close() error
}
```

### Client 接口

WebSocket 客户端抽象：

```go
type Client interface {
    ID() string
    Send(msg *Message) error
    Close() error
    Context() context.Context
    SetContext(ctx context.Context)
    Metadata() map[string]any
    SetMetadata(key string, value any)
}
```

### 创建与连接

```go
// 创建 Hub，传入消息处理器和可选中间件
hub := websocket.NewHub(handler, middlewares...)

// HTTP 处理函数（用于路由注册）
http.HandleFunc("/ws", websocket.HTTPHandler(hub, config))

// 或手动升级连接
err := websocket.ServeWS(hub, w, r, config)
```

### Config 配置

```go
config := websocket.DefaultConfig()
```

| 字段                | 默认值  | 说明             |
| ------------------- | ------- | ---------------- |
| `ReadBufferSize`    | `1024`  | 读缓冲区大小     |
| `WriteBufferSize`   | `1024`  | 写缓冲区大小     |
| `MaxMessageSize`    | `512KB` | 最大消息大小     |
| `WriteTimeout`      | `10s`   | 写超时           |
| `ReadTimeout`       | `60s`   | 读超时           |
| `PingInterval`      | `30s`   | Ping 发送间隔    |
| `PongTimeout`       | `60s`   | Pong 等待超时    |
| `EnableCompression` | `true`  | 是否启用压缩     |
| `CheckOrigin`       | 允许全部 | 跨域检查函数    |

### Message 消息

```go
type Message struct {
    Type      MessageType  // 消息类型
    Data      []byte       // 消息数据
    ClientID  string       // 发送者 ID
    Timestamp time.Time    // 时间戳
}
```

### MessageType 消息类型

| 常量            | 值   | 说明       |
| --------------- | ---- | ---------- |
| `TextMessage`   | `1`  | 文本消息   |
| `BinaryMessage` | `2`  | 二进制消息 |
| `CloseMessage`  | `8`  | 关闭消息   |
| `PingMessage`   | `9`  | Ping 消息  |
| `PongMessage`   | `10` | Pong 消息  |

### 内置中间件

```go
// 日志中间件：记录消息接收和处理耗时
websocket.LoggingMiddleware(log)

// Recovery 中间件：捕获 handler panic
websocket.RecoveryMiddleware(log)

// 限流中间件：限制每个客户端的消息频率
websocket.RateLimitMiddleware(100, time.Minute)

// 消息大小限制中间件
websocket.MessageSizeMiddleware(1024 * 1024)

// 认证中间件
websocket.AuthMiddleware(validateTokenFunc)
```

### 自定义中间件

```go
type Handler func(client Client, msg *Message)
type Middleware func(Handler) Handler

myMiddleware := func(next websocket.Handler) websocket.Handler {
    return func(client websocket.Client, msg *websocket.Message) {
        // 前置逻辑
        next(client, msg)
        // 后置逻辑
    }
}
```

## 许可证

详见项目根目录 LICENSE 文件。
