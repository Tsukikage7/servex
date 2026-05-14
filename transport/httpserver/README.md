# httpserver

`httpserver` 包提供 HTTP 服务器实现，支持中间件链、健康检查、链路追踪、认证、pprof 等功能。

## 功能特性

- 基于标准库 `net/http` 实现，支持任意 `http.Handler`
- 内置中间件：Recovery、Trace、ClientIP、Auth、Health
- 支持 pprof 性能分析端点（可选认证）
- Endpoint 模式将业务逻辑与 HTTP 传输解耦
- Codec 集成，根据 Content-Type/Accept 自动选择编解码器
- 实现 `transport.HealthCheckable` 接口

## 安装

```bash
go get github.com/Tsukikage7/servex/v2/transport/httpserver
```

## API

### Server

```go
func New(handler http.Handler, opts ...Option) *Server
func (s *Server) Start(ctx context.Context) error
func (s *Server) Stop(ctx context.Context) error
func (s *Server) Name() string
func (s *Server) Addr() string
func (s *Server) Handler() http.Handler
func (s *Server) Health() *health.Health
func (s *Server) HealthEndpoint() *transport.HealthEndpoint
```

### 配置选项

| 选项                 | 默认值         | 说明                         |
| -------------------- | -------------- | ---------------------------- |
| `WithLogger`         | -              | 日志记录器（必需）           |
| `WithName`           | `HTTP`         | 服务器名称                   |
| `WithAddr`           | `:8080`        | 监听地址                     |
| `WithTimeout`        | `30s/30s/120s` | 超时设置（read/write/idle）  |
| `WithRecovery`       | `false`        | 启用 panic 恢复              |
| `WithTrace`          | -              | 启用链路追踪                 |
| `WithClientIP`       | `false`        | 启用客户端 IP 提取           |
| `WithAuth`           | -              | 启用认证，可指定公开路径     |
| `WithProfiling`      | -              | 启用 pprof 端点              |
| `WithProfilingAuth`  | -              | 启用带认证的 pprof 端点      |
| `WithHealthTimeout`  | `5s`           | 健康检查超时                 |
| `WithHealthChecker`  | -              | 添加就绪检查器               |

### 类型安全 Handler（推荐）

`Handle` 和 `HandleWith` 是推荐的路由注册方式，无需手写解码/编码/错误处理样板。

```go
// Handle — 适合 POST/PUT/PATCH（请求体 JSON 解码）
router.POST("/users", httpserver.Handle(
    func(ctx context.Context, req CreateUserReq) (*UserResp, error) {
        return svc.CreateUser(ctx, req)
        // 返回：{"code":0,"message":"成功","data":{"id":1,"name":"Alice"}}
    },
))

// HandleWith — 适合 GET/DELETE（路径参数/查询字符串）
router.GET("/users/{id}", httpserver.HandleWith(
    func(ctx context.Context, r *http.Request) (GetUserReq, error) {
        return GetUserReq{ID: r.PathValue("id")}, nil
    },
    func(ctx context.Context, req GetUserReq) (*UserResp, error) {
        return svc.GetUser(ctx, req.ID)
    },
))
```

**自动包装规则**：
- 成功响应 → `{"code":0,"message":"成功","data":{...}}`
- 若返回值已是 `response.Response[T]` / `response.PagedResponse[T]`，不再二次包装
- 错误 → `{"code":xxxxx,"message":"..."}` + 正确 HTTP 状态码

**Validatable 接口**（可选）：请求结构体实现 `Validate() error`，`Handle`/`HandleWith` 自动在解码后调用：

```go
type CreateUserReq struct {
    Name string `json:"name"`
}

func (r *CreateUserReq) Validate() error {
    if r.Name == "" {
        return response.CodeInvalidParam.ToError()
    }
    return nil
}
```

### Router

`Router` 是 `http.ServeMux` 的轻量封装，提供路由分组与多级中间件：

```go
router := httpserver.NewRouter()

// 公开路由
router.POST("/login", httpserver.Handle(loginHandler))

// 带认证的 API 分组
api := router.Group("/api/v1", jwtMiddleware)
api.GET("/users/{id}", httpserver.HandleWith(decodeID, getUser))
api.POST("/users", httpserver.Handle(createUser))

// 嵌套分组 + 额外中间件
admin := api.Group("/admin", adminOnlyMiddleware)
admin.DELETE("/users/{id}", httpserver.HandleWith(decodeID, deleteUser))

srv := httpserver.New(router,
    httpserver.WithLogger(log),
    httpserver.WithRecovery(),
)
```

#### Router API

| 方法                                       | 说明                           |
| ------------------------------------------ | ------------------------------ |
| `NewRouter(mws ...Middleware)`             | 创建根路由器，可传入全局中间件 |
| `Use(mws ...Middleware)`                   | 向当前路由器追加中间件         |
| `Group(prefix, mws ...Middleware) *Router` | 创建子路由分组（继承父中间件） |
| `GET/POST/PUT/PATCH/DELETE(path, handler)` | 注册对应方法路由               |
| `Handle(pattern, handler)`                 | 注册任意方法路由（支持 METHOD /path 格式） |

### EndpointHandler

将 `endpoint.Endpoint` 包装为 `http.Handler`（底层 API，适合复杂场景）：

```go
handler := httpserver.NewEndpointHandler(
    getUserEndpoint,
    decodeGetUserRequest,
    httpserver.EncodeJSONResponse,
    httpserver.WithBefore(extractAuthToken),
    httpserver.WithResponse(),
)
mux.Handle("/users/{id}", handler)
```

#### EndpointHandler 选项

| 选项               | 说明                                                 |
| ------------------ | ---------------------------------------------------- |
| `WithBefore`       | 添加请求前处理函数                                   |
| `WithAfter`        | 添加响应后处理函数                                   |
| `WithErrorEncoder` | 自定义错误编码器                                     |
| `WithResponse`     | 启用统一响应格式错误编码器（含 i18n 错误消息）       |
| `WithValidate`     | 启用自动校验，可传入自定义校验函数，默认检测 Validatable 接口 |

### 函数类型

```go
type DecodeRequestFunc func(ctx context.Context, r *http.Request) (any, error)
type EncodeResponseFunc func(ctx context.Context, w http.ResponseWriter, response any) error
type RequestFunc func(ctx context.Context, r *http.Request) context.Context
type Middleware = func(http.Handler) http.Handler
```

### Codec 集成

根据请求头自动选择编解码器（JSON、XML、Protobuf）：

```go
handler := httpserver.NewEndpointHandler(
    myEndpoint,
    httpserver.DecodeCodecRequest[MyRequest](),
    httpserver.EncodeCodecResponse,
    httpserver.WithBefore(httpserver.WithRequest()),
)
```

### 响应编码

```go
// JSON 响应
httpserver.EncodeJSONResponse(ctx, w, resp)

// 基于 Accept 头自动选择编解码器
httpserver.EncodeCodecResponse(ctx, w, resp)
```

## 中间件执行顺序

从外到内：

```
Profiling -> Recovery -> Trace -> Auth -> ClientIP -> Health -> 业务逻辑
```

## 许可证

详见项目根目录 LICENSE 文件。
