# servex 统一错误处理

## Error 类型 -- 业务错误定义

```go
import "github.com/Tsukikage7/servex/v2/errors"

// 定义错误常量（通常在包级别）
var (
    ErrNotFound    = errors.NewWithKind(
                        404001,
                        "not_found",
                        "资源不存在",
                        errors.KindNotFound,
                    )

    ErrPermission  = errors.NewWithKind(
                        403001,
                        "permission_denied",
                        "权限不足",
                        errors.KindPermissionDenied,
                    )

    ErrInternal    = errors.NewWithKind(
                        500001,
                        "internal",
                        "服务器内部错误",
                        errors.KindInternal,
                    )
)
```

**Error 结构体字段：**
- `Code` — 业务错误码（int）
- `Key` — 错误标识（string，如 "not_found"）
- `Message` — 面向用户的错误消息
- `Kind` — 业务错误语义，HTTP 状态码与 gRPC Code 由它统一推导
- `Metadata` — 附加元数据

## 错误传播 -- 包装与提取

```go
// 包装底层错误（返回新实例，不修改原定义）
err := ErrNotFound.WithCause(fmt.Errorf("数据库查询为空"))

// 附加元数据
err = ErrNotFound.WithMeta("resource_id", "123")

// 覆盖消息
err = ErrNotFound.WithMessage("用户不存在")

// 从 error 链中提取 *Error
e, ok := errors.FromError(err)

// 按 Code 比较（支持 errors.Is 语义）
errors.CodeIs(err, ErrNotFound) // true

// 标准库兼容
stderrors.Is(err, ErrNotFound)  // true（按 Code 比较）
```

**注意：** `WithCause`/`WithMeta`/`WithMessage` 均返回浅拷贝新实例，不会修改包级错误常量。

## HTTP 错误响应

```go
import "github.com/Tsukikage7/servex/v2/errors"

// 提取 HTTP 状态码（默认 500）
status := errors.ToHTTPStatus(err)

// 写入 HTTP 响应（JSON 格式）
errors.WriteError(w, ErrNotFound)
// 响应体: {"code":404001,"key":"not_found","message":"资源不存在"}

// 从任意 error 写入响应
errors.WriteErrorFrom(w, err)
// 若 err 不是 *Error，返回 500 + 通用内部错误 message，并记录原始错误
```

**HTTP JSON 响应格式：**
```json
{
    "code": 404001,
    "key": "not_found",
    "message": "资源不存在",
    "metadata": {"resource_id": "123"}
}
```

## gRPC 错误映射

`response` 包统一委托 `errors/grpcx.ToGRPCStatus` 输出同一种 JSON 格式。
业务 API 推荐从 `response.Code.ToError()` 创建错误，再由 response/gateway/http server 适配层自动处理。
更底层的包可以直接使用 `errors.NewWithKind(...)`，由 `Kind` 统一推导 HTTP/gRPC 映射。

**`response` 包**（推荐用于 response 体系）：

```go
import "github.com/Tsukikage7/servex/v2/transport/response"

// error → gRPC Status（message 嵌入统一 JSON，保留细粒度业务 Code）
st := response.GRPCStatus(err)
// st.Message() = `{"code":30002,"key":"error.missing_param","message":"缺少必需参数","kind":"invalid_argument"}`

// gRPC Status → Code（优先读取 JSON，回退到 gRPC code 映射）
code := response.FromGRPCStatus(st)
// code.Num = 30002（不会被降级为 30001）

// gRPC 拦截器
grpcserver.New(
    grpcserver.WithUnaryInterceptor(response.UnaryServerInterceptor()),
)
```

**`errors` 包**（推荐用于 errors 体系）：

```go
import errorsgrpcx "github.com/Tsukikage7/servex/v2/errors/grpcx"

// *Error → gRPC Status（Detail 为 JSON 序列化的错误信息）
st := errorsgrpcx.ToGRPCStatus(err)

// gRPC Status → *Error
e := errorsgrpcx.FromGRPCStatus(st)

// gRPC 一元拦截器
grpcserver.New(
    grpcserver.WithUnaryInterceptor(errorsgrpcx.UnaryServerInterceptor()),
)
```

**gRPC 映射流程（response 体系）：**
1. 业务层返回 `response.Code.ToError()` 生成的统一错误
2. 拦截器调用 `GRPCStatus(err)` 转为 gRPC Status，message 嵌入 JSON
3. gRPC-gateway `GatewayErrorHandler` 调用 `FromGRPCStatus` 还原完整 Code
4. 若不是 servex 统一错误格式（非 servex 来源），按原生 gRPC code 映射

## 错误码段分配（v2.0.6+）

servex 所有对外暴露的错误统一使用 `errors.NewWithKind(code, key, msg, kind)` 定义，
HTTP 状态码和 gRPC Code 由 `Kind` 统一推导。错误码段分配如下：

| 码段 | 范围 | 领域 | 示例 |
|------|------|------|------|
| 20xxx | 20001-20999 | 认证/授权（auth） | `ErrUnauthenticated(20001)`, `ErrForbidden(20002)` |
| 201xx | 20101-20199 | JWT | `ErrTokenInvalid(20101)`, `ErrTokenRevoked(20102)` |
| 203xx | 20301-20399 | RBAC | `ErrRoleNotFound(20301)`, `ErrPermissionDenied(20303)` |
| 60xxx | 60001-60999 | 传输层（transport） | `ErrRequestFailed(60001)`, `ErrConnectionFailed(60101)` |
| 602xx | 60201-60299 | GraphQL | `ErrNilSchema(60201)` |
| 70xxx | 70001-70999 | 通知（notify） | `ErrNilMessage(70001)`, `ErrEmptyChannel(70002)` |
| 700xx | 70061-70079 | notify/discord | `ErrEmptyToken(70061)`, `ErrSessionCreate(70062)` |
| 700xx | 70071-70079 | notify/telegram | `ErrEmptyToken(70071)`, `ErrBotAPICreate(70072)` |
| 80xxx | 80001-80999 | OAuth2 | `ErrInvalidState(80001)`, `ErrExchangeFailed(80002)` |
| 90xxx | 90001-90999 | 服务发现（discovery） | `ErrNilConfig(90001)`, `ErrNotFound(90012)` |

**自定义业务错误码建议：** 使用 `100000+` 起始，避免与框架错误码冲突。

## 完整示例 -- 定义 + 使用

```go
import "github.com/Tsukikage7/servex/v2/errors"

// errors/codes.go — 错误码定义
var (
    ErrUserNotFound = errors.NewWithKind(100404, "user_not_found", "用户不存在", errors.KindNotFound)

    ErrUserExists = errors.NewWithKind(100409, "user_exists", "用户已存在", errors.KindConflict)
)

// service 层
func GetUser(ctx context.Context, id string) (*User, error) {
    user, err := repo.FindByID(ctx, id)
    if err != nil {
        return nil, ErrUserNotFound.WithCause(err).WithMeta("user_id", id)
    }
    return user, nil
}

// HTTP handler
func handleGetUser(w http.ResponseWriter, r *http.Request) {
    user, err := svc.GetUser(r.Context(), r.PathValue("id"))
    if err != nil {
        errors.WriteErrorFrom(w, err)
        return
    }
    json.NewEncoder(w).Encode(user)
}
```
