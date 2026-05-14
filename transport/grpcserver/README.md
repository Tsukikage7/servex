# grpcserver

`grpcserver` 包提供 gRPC 服务器实现，支持服务注册、拦截器链、健康检查、链路追踪、认证和公开方法自动发现。

## 功能特性

- 基于 `google.golang.org/grpc` 实现
- 通过 `Registrar` 接口统一注册 gRPC 服务
- 内置健康检查（gRPC Health Checking Protocol）
- 支持反射、Keepalive、Recovery、Trace、Auth、ClientIP 等拦截器
- 支持 proto option 自动发现公开方法
- Endpoint 模式将业务逻辑与 gRPC 传输解耦
- 实现 `transport.HealthCheckable` 接口

## 安装

```bash
go get github.com/Tsukikage7/servex/v2/transport/grpcserver
```

## API

### Server

```go
func New(opts ...Option) *Server
func (s *Server) Start(ctx context.Context) error
func (s *Server) Stop(ctx context.Context) error
func (s *Server) Register(services ...Registrar) *Server
func (s *Server) GRPCServer() *grpc.Server
func (s *Server) Health() *health.Health
func (s *Server) HealthEndpoint() *transport.HealthEndpoint
func (s *Server) Name() string
func (s *Server) Addr() string
```

### Registrar 接口

业务服务需实现此接口以注册到 gRPC 服务器：

```go
type Registrar interface {
    RegisterGRPC(server *grpc.Server)
}
```

### 配置选项

| 选项                    | 默认值     | 说明                         |
| ----------------------- | ---------- | ---------------------------- |
| `WithLogger`            | -          | 日志记录器（必需）           |
| `WithName`              | `gRPC`     | 服务器名称                   |
| `WithAddr`              | `:9090`    | 监听地址                     |
| `WithReflection`        | `true`     | 是否启用 gRPC 反射           |
| `WithKeepalive`         | `60s, 20s` | Keepalive 参数               |
| `WithUnaryInterceptor`  | -          | 添加一元拦截器               |
| `WithStreamInterceptor` | -          | 添加流拦截器                 |
| `WithServerOption`      | -          | 自定义 gRPC ServerOption     |
| `WithConfig`            | -          | 从 GRPCConfig 结构体加载配置 |
| `WithClientIP`          | -          | 启用客户端 IP 提取           |
| `WithHealthTimeout`     | `5s`       | 健康检查超时                 |
| `WithReadinessChecker`  | -          | 添加就绪检查器               |
| `WithLivenessChecker`   | -          | 添加存活检查器               |

### EndpointHandler

将 `endpoint.Endpoint` 包装为 gRPC Handler：

```go
handler := grpcserver.NewEndpointHandler(
    getUserEndpoint,
    decodeGetUserRequest,
    encodeGetUserResponse,
    grpcserver.WithBefore(extractAuthFromMD),
    grpcserver.WithResponse(),
)
```

在 gRPC 服务实现中调用：

```go
func (s *userServer) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
    _, resp, err := s.getUserHandler.ServeGRPC(ctx, req)
    if err != nil {
        return nil, err
    }
    return resp.(*pb.GetUserResponse), nil
}
```

### Handler 接口

```go
type Handler interface {
    ServeGRPC(ctx context.Context, request any) (context.Context, any, error)
}
```

### Passthrough 直通辅助函数

适合 Endpoint 直接操作 proto 消息、无需中间转换的场景：

```go
// PassthroughHandler — 最简用法，等价于 NewEndpointHandler(e, PassthroughDecode, PassthroughEncode, opts...)
handler := grpcserver.PassthroughHandler(
    func(ctx context.Context, req any) (any, error) {
        r := req.(*pb.GetUserRequest)
        user, err := svc.GetUser(ctx, r.Id)
        if err != nil {
            return nil, err
        }
        return &pb.GetUserResponse{Id: user.Id, Name: user.Name}, nil
    },
    grpcserver.WithResponse(),
)
```

也可单独使用：

```go
// PassthroughDecode — 将 proto 请求原样传入 Endpoint
// PassthroughEncode — 将 Endpoint 返回值原样作为 gRPC 响应
handler := grpcserver.NewEndpointHandler(e,
    grpcserver.PassthroughDecode,
    grpcserver.PassthroughEncode,
)
```

## 许可证

详见项目根目录 LICENSE 文件。
