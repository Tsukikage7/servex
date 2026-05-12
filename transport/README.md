# transport

`transport` 包定义了 servex 传输层的核心接口和配置结构，是所有服务器与客户端实现的基础抽象层。

## 功能特性

- 定义统一的 `Server` 接口，规范服务器生命周期管理
- 提供 `HealthCheckable` 接口，支持健康检查能力扩展
- 内置 TCP、HTTP、gRPC 三种健康检查类型
- 提供 HTTP、gRPC、Gateway 三种服务器配置结构，支持 JSON/YAML/mapstructure 标签

## 安装

```bash
go get github.com/Tsukikage7/servex
```

## API

### Server 接口

所有服务器实现均需满足此接口：

```go
type Server interface {
    Start(ctx context.Context) error  // 启动服务器
    Stop(ctx context.Context) error   // 停止服务器
    Name() string                     // 返回服务器名称
    Addr() string                     // 返回监听地址
}
```

### HealthCheckable 接口

扩展 `Server` 接口，增加健康检查能力：

```go
type HealthCheckable interface {
    Server
    Health() *health.Health               // 返回健康检查管理器
    HealthEndpoint() *HealthEndpoint      // 返回健康检查端点信息
}
```

### HealthCheckType

健康检查类型枚举：

| 常量                  | 值     | 说明            |
| --------------------- | ------ | --------------- |
| `HealthCheckTypeTCP`  | `tcp`  | TCP 端口检查    |
| `HealthCheckTypeHTTP` | `http` | HTTP 端点检查   |
| `HealthCheckTypeGRPC` | `grpc` | gRPC 健康检查   |

### HealthEndpoint 结构

```go
type HealthEndpoint struct {
    Type HealthCheckType  // 检查类型
    Addr string           // 检查地址
    Path string           // 检查路径（仅 HTTP 类型使用）
}
```

### 配置结构

#### HTTPConfig

| 字段           | 类型            | 说明       |
| -------------- | --------------- | ---------- |
| `Name`         | `string`        | 服务器名称 |
| `Addr`         | `string`        | 监听地址   |
| `ReadTimeout`  | `time.Duration` | 读取超时   |
| `WriteTimeout` | `time.Duration` | 写入超时   |
| `IdleTimeout`  | `time.Duration` | 空闲超时   |
| `PublicPaths`  | `[]string`      | 公开路径   |

#### GRPCConfig

| 字段               | 类型            | 说明             |
| ------------------ | --------------- | ---------------- |
| `Name`             | `string`        | 服务器名称       |
| `Addr`             | `string`        | 监听地址         |
| `EnableReflection` | `bool`          | 是否启用反射     |
| `KeepaliveTime`    | `time.Duration` | Keepalive 间隔   |
| `KeepaliveTimeout` | `time.Duration` | Keepalive 超时   |

#### GatewayConfig

| 字段            | 类型            | 说明           |
| --------------- | --------------- | -------------- |
| `Name`          | `string`        | 服务器名称     |
| `GRPCAddr`      | `string`        | gRPC 监听地址  |
| `HTTPAddr`      | `string`        | HTTP 监听地址  |
| `KeepaliveTime` | `time.Duration` | Keepalive 间隔 |

## 目录结构

```
transport/
├── httpserver/     # HTTP 服务器
├── grpcserver/     # gRPC 服务器
├── grpcclient/     # gRPC 客户端
├── httpclient/     # HTTP 客户端
├── gateway/        # gRPC-Gateway 双协议服务器
├── connectserver/  # Connect RPC 服务器（选配）
├── health/         # 健康检查（HTTP + gRPC）
├── websocket/      # WebSocket 实时通信
├── response/       # 统一响应格式
└── server.go       # 核心接口定义
```

## 许可证

详见项目根目录 LICENSE 文件。
