# echoserver

`echoserver` 包为 [Echo](https://github.com/labstack/echo) 框架提供类型安全的 Handler 适配器。

在完整保留 Echo 原生路由与中间件体系的前提下，统一处理请求解码、参数校验、响应包装与 i18n 错误消息。

## 安装

```bash
go get github.com/Tsukikage7/servex/transport/echoserver
```

## API

### Handle

适用于请求体为 JSON 的场景（POST / PUT / PATCH）：

```go
e.POST("/users", echoserver.Handle(
    func(ctx context.Context, req CreateUserReq) (*UserResp, error) {
        return svc.CreateUser(ctx, req)
        // 返回：{"code":0,"message":"成功","data":{"id":1,"name":"Alice"}}
    },
))
```

### HandleWith

适用于需要从路径参数、查询字符串等位置提取请求的场景（GET / DELETE）：

```go
e.GET("/users/:id", echoserver.HandleWith(
    func(c echo.Context) (GetUserReq, error) {
        return GetUserReq{ID: c.Param("id")}, nil
    },
    func(ctx context.Context, req GetUserReq) (*UserResp, error) {
        return svc.GetUser(ctx, req.ID)
    },
))
```

## 响应包装规则

| 情况                                              | 输出                                              |
| ------------------------------------------------- | ------------------------------------------------- |
| 成功，返回普通结构体                              | `{"code":0,"message":"成功","data":{...}}`        |
| 成功，返回 `response.Response[T]` / `PagedResponse[T]` | 原样输出，不二次包装                         |
| 错误（业务 error / `response.Code`）              | `{"code":xxxxx,"message":"..."}` + 对应 HTTP 状态码 |

错误消息自动根据 `Accept-Language` 请求头翻译（内置中/英文）。

## Validatable 自动校验

请求结构体可实现 `Validate() error` 接口，`Handle`/`HandleWith` 在解码后自动调用：

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

## 完整示例

```go
import (
    "github.com/labstack/echo/v4"
    "github.com/Tsukikage7/servex/transport/echoserver"
    "github.com/Tsukikage7/servex/transport/response"
)

func main() {
    e := echo.New()

    e.POST("/users", echoserver.Handle(createUser))
    e.GET("/users/:id", echoserver.HandleWith(decodeGetUser, getUser))

    e.Start(":8080")
}

func createUser(ctx context.Context, req CreateUserReq) (*UserResp, error) {
    // 业务逻辑...
    return &UserResp{ID: 1}, nil
}

func decodeGetUser(c echo.Context) (GetUserReq, error) {
    return GetUserReq{ID: c.Param("id")}, nil
}

func getUser(ctx context.Context, req GetUserReq) (*UserResp, error) {
    return svc.GetUser(ctx, req.ID)
}
```

## 许可证

详见项目根目录 LICENSE 文件。
