# Response 统一响应体包

提供微服务 API 的统一响应格式，支持 HTTP 和 gRPC 协议，内置错误码体系和分页支持。

## 特性

- 泛型响应体 `Response[T]`
- 数字错误码体系（含 i18n 消息键）
- HTTP/gRPC 状态码自动映射
- 内置分页响应 `PagedResponse[T]`
- 与 `gateway` 包无缝集成
- 内部错误信息自动隐藏
- i18n 本地化错误消息（内置中/英文，可扩展）

## 响应格式

### 标准响应

```json
{
  "code": 0,
  "message": "成功",
  "data": { ... }
}
```

### 分页响应

```json
{
  "code": 0,
  "message": "成功",
  "data": [...],
  "pagination": {
    "page": 1,
    "page_size": 20,
    "total": 100,
    "total_pages": 5
  }
}
```

### 错误响应

```json
{
  "code": 40001,
  "message": "用户不存在"
}
```

## 状态码映射

| 业务错误码               | HTTP 状态码 | gRPC 状态码       |
| ------------------------ | ----------- | ----------------- |
| `CodeSuccess`            | 200         | OK                |
| `CodeInvalidParam`       | 400         | InvalidArgument   |
| `CodeUnauthorized`       | 401         | Unauthenticated   |
| `CodeForbidden`          | 403         | PermissionDenied  |
| `CodeNotFound`           | 404         | NotFound          |
| `CodeAlreadyExists`      | 409         | AlreadyExists     |
| `CodeResourceExhausted`  | 429         | ResourceExhausted |
| `CodeInternal`           | 500         | Internal          |
| `CodeServiceUnavailable` | 503         | Unavailable       |

## API 参考

### 响应构建

| 函数                            | 说明                  |
| ------------------------------- | --------------------- |
| `OK[T](data)`                   | 成功响应              |
| `OKWithMessage[T](data, msg)`   | 带消息的成功响应      |
| `Fail[T](code)`                 | 失败响应              |
| `FailWithMessage[T](code, msg)` | 带消息的失败响应      |
| `FailWithError[T](err)`         | 从 error 创建失败响应 |
| `Paged[T](result)`              | 分页响应              |
| `PagedFail[T](code)`            | 分页失败响应          |

### 错误处理

| 函数                             | 说明                             |
| -------------------------------- | -------------------------------- |
| `NewCodeWithKind(num, key, message, kind)` | 创建自定义错误码，HTTP/gRPC 由 Kind 推导 |
| `Code.ToError()`                 | 转换为统一错误类型               |
| `Code.ToError().WithMessage(msg)` | 带消息的业务错误                 |
| `Code.ToError().WithCause(err)`  | 包装底层错误                     |
| `Code.WithHTTPStatus(status)`    | 覆盖 HTTP 状态码                 |
| `ExtractCode(err)`               | 提取错误码                       |
| `ExtractMessage(err)`            | 提取错误消息（内部错误隐藏详情） |
| `ExtractMessageUnsafe(err)`      | 提取完整错误消息（仅用于日志）   |

### gRPC 集成

| 函数                        | 说明                                                         |
| --------------------------- | ------------------------------------------------------------ |
| `GRPCStatus(err)`           | 错误转 gRPC Status，message 嵌入 JSON 保留细粒度业务 Code    |
| `GRPCError(err)`            | 转换为 gRPC error                                            |
| `FromGRPCStatus(s)`         | 从 gRPC Status 提取 Code，优先读取 JSON payload              |
| `FromGRPCError(err)`        | 从 gRPC error 提取 Code                                      |
| `UnaryServerInterceptor()`  | gRPC 一元拦截器                                              |
| `StreamServerInterceptor()` | gRPC 流拦截器                                                |

**细粒度 Code 保留机制：** 同一 gRPC code（如 `InvalidArgument`）可对应多个业务 Code（30001/30002/30003）。
`GRPCStatus` 将完整 Code 信息以 JSON 嵌入 message，`FromGRPCStatus` 优先从中恢复，不再依赖粗粒度反向映射。
非 servex 来源的 gRPC 错误按原生 gRPC code 映射。

### HTTP 集成

| 函数                                  | 说明                                   |
| ------------------------------------- | -------------------------------------- |
| `WriteSuccess[T](w, data)`            | 写入成功响应                           |
| `WriteFail(w, code)`                  | 写入失败响应                           |
| `WriteError(w, err, langs...)`        | 写入错误响应，可传入语言偏好           |
| `WriteLocalizedError(w, r, err)`      | 根据请求 `Accept-Language` 写入错误响应 |
| `WritePaged[T](w, resp)`              | 写入分页响应                           |

### i18n 本地化

`response` 包内置中英文错误消息，通过 `Accept-Language` 请求头自动选择语言。

```go
// 获取本地化错误消息（通常由框架适配层自动调用）
msg := response.LocalizedMessage(err, "en-US,en;q=0.9")  // → "Resource not found"
msg := response.LocalizedMessage(err, "zh-CN")            // → "资源不存在"

// 裸 net/http handler 中直接写入本地化错误响应
_ = response.WriteLocalizedError(w, r, err)
```

业务层显式传入的非内部错误消息优先于 i18n 翻译；内部错误（5xxxx/6xxxx）始终返回通用本地化消息，避免暴露敏感细节。

**替换/扩展消息包**（在应用启动时调用一次）：

```go
bundle := i18n.NewBundle(language.Chinese)
bundle.LoadMessages(language.Chinese, map[string]string{
    "error.not_found": "找不到该资源",
    "my.custom.error": "自定义错误消息",
})
bundle.LoadMessages(language.English, map[string]string{
    "error.not_found": "The resource was not found",
    "my.custom.error": "Custom error message",
})
response.SetBundle(bundle)
```

**自定义错误码 + i18n 键**：

```go
import servexerr "github.com/Tsukikage7/servex/v2/errors"

var ErrUserBanned = response.NewCodeWithKind(
    40010,
    "error.user_banned",  // i18n 消息键
    "账号已封禁",           // 未命中 i18n 时的回退值
    servexerr.KindPermissionDenied,
)
```

需要非标准 HTTP 映射时，再显式覆盖：

```go
var ErrUpstreamGateway = response.NewCodeWithKind(
    60010,
    "error.upstream_gateway",
    "上游服务错误",
    servexerr.KindUnavailable,
).WithHTTPStatus(http.StatusBadGateway)
```

**内置消息键**（中英文均内置）：

| 键                    | 中文默认值       | 英文默认值              |
| --------------------- | ---------------- | ----------------------- |
| `success`             | 成功             | Success                 |
| `error.unauthorized`  | 未授权           | Unauthorized            |
| `error.forbidden`     | 禁止访问         | Forbidden               |
| `error.invalid_param` | 参数无效         | Invalid parameter       |
| `error.not_found`     | 资源不存在       | Resource not found      |
| `error.internal`      | 服务器内部错误   | Internal server error   |
| `error.unavailable`   | 服务不可用       | Service unavailable     |
| （更多见 `bundle.go`）  |                  |                         |
