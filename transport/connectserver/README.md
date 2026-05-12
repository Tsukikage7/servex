# connectserver

`connectserver` 提供可选的 Connect RPC HTTP 服务器，用于需要前端按 proto 生成强类型 RPC client 的场景。它不会替代 `transport/gateway`，也不会默认启用。

## 协议定位

- 默认前端协议仍推荐 `transport/gateway` 的 HTTP/JSON。
- `connectserver` 面向 Connect-Web / gRPC-Web / gRPC 兼容访问。
- 服务需要显式实现 `RegisterConnect` 后才会挂载。

## 示例

```go
type UserService struct{}

func (s *UserService) RegisterConnect(mux *http.ServeMux, opts ...connect.HandlerOption) {
	path, handler := userv1connect.NewUserServiceHandler(s, opts...)
	mux.Handle(path, handler)
}

srv := connectserver.New(
	connectserver.WithLogger(log),
	connectserver.WithAddr(":8081"),
	connectserver.WithRecovery(),
	connectserver.WithLogging("/healthz", "/readyz"),
)
srv.Register(&UserService{})
```

## 设计约束

- Connect 是选配 transport，未导入 `transport/connectserver` 的服务不受影响。
- `WithHandlerOptions` 会传给生成的 Connect handler，可用于拦截器、压缩、协议约束等能力。
- CORS、认证、限流等 HTTP 中间件通过 `WithMiddlewares` 显式接入。
