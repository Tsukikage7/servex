# servex OpenAPI 文档生成（3.1）

## 核心用法

```go
import "github.com/Tsukikage7/servex/v2/openapi"

// 1. 创建 Registry
reg := openapi.NewRegistry(
    openapi.WithInfo("My Service", "1.0.0", "订单服务 API"),
    openapi.WithServer("https://api.example.com"),
)

// 2. 注册端点（可以和路由注册放在一起）
reg.Add(openapi.POST("/orders").
    Summary("创建订单").
    Description("创建一个新订单").
    Tags("orders").
    OperationID("createOrder").
    Request(CreateOrderRequest{}).
    Response(CreateOrderResponse{}).
    Build(),
)

reg.Add(openapi.GET("/orders/{id}").
    Summary("查询订单").
    Tags("orders").
    Response(Order{}).
    Build(),
)

reg.Add(openapi.DELETE("/orders/{id}").
    Summary("删除订单").
    Tags("orders").
    Deprecated(true).
    Build(),
)

// 3. 挂载文档端点
mux.Handle("/openapi.json", reg.ServeJSON())
mux.Handle("/openapi.yaml", reg.ServeYAML())
```

## Schema 生成规则

通过 struct tag 反射自动生成 JSON Schema：

```go
type CreateOrderRequest struct {
    UserID  string  `json:"user_id" validate:"required" description:"用户ID"`
    Amount  float64 `json:"amount"  validate:"required,min=0.01" description:"金额"`
    Remark  string  `json:"remark,omitempty" description:"备注"`
    Count   int     `json:"count"   validate:"min=1,max=100"`
}
```

| struct tag | 作用 | 示例 |
|-----------|------|------|
| `json` | 字段名 + omitempty | `json:"user_id"` |
| `validate` | required + min/max 约束 | `validate:"required,min=0.01"` |
| `description` | 字段描述 | `description:"用户ID"` |

**required 判定**：有 `validate:"required"` 且 json tag 不含 `omitempty`。

**类型映射**：
- `string` → `"string"`
- `int/int64/uint` → `"integer"`
- `float32/float64` → `"number"`
- `bool` → `"boolean"`
- `[]T` → `{"type":"array","items":{...}}`
- `time.Time` → `{"type":"string","format":"date-time"}`
- `*T` → 解引用后按 T 的类型处理
- 嵌套 struct → 递归生成

## Builder 方法

| 构造函数 | 用法 |
|---------|------|
| `GET(path)` | `openapi.GET("/users/{id}")` |
| `POST(path)` | `openapi.POST("/users")` |
| `PUT(path)` | `openapi.PUT("/users/{id}")` |
| `DELETE(path)` | `openapi.DELETE("/users/{id}")` |
| `PATCH(path)` | `openapi.PATCH("/users/{id}")` |

| 链式方法 | 说明 |
|---------|------|
| `.Summary(s)` | 端点摘要 |
| `.Description(s)` | 详细描述 |
| `.Tags(tags...)` | 分组标签 |
| `.OperationID(id)` | 操作 ID |
| `.Deprecated(bool)` | 标记废弃 |
| `.Request(v)` | 请求体类型（自动生成 Schema） |
| `.Response(v)` | 响应体类型 |
| `.Errors(types...)` | 错误响应类型 |
| `.Build()` | 构建 Operation |

## 手动生成 Schema

```go
schema := openapi.SchemaFrom(MyStruct{})
// schema.Type, schema.Properties, schema.Required, etc.
```

## Webhooks

```go
// 注册 Webhook 定义（OpenAPI 3.1）
reg.AddWebhook("orderCreated",
    openapi.POST("").
        Summary("订单创建回调").
        Description("当订单创建成功时，向订阅方发送回调通知").
        Tags("webhooks").
        Request(OrderWebhookPayload{}).
        Response(WebhookAck{}).
        Build(),
)

// Webhook 会出现在 openapi.json 的 webhooks 字段中
mux.Handle("/openapi.json", reg.ServeJSON())
```

**关键 API：**
- `reg.AddWebhook(name, operation)` — 注册 Webhook 定义
- Webhook 请求/响应类型遵循与普通端点相同的 Schema 生成规则
