# openapi

`github.com/Tsukikage7/servex/v2/openapi` -- Code-first OpenAPI 3.0 文档生成。

## 概述

openapi 包提供 Code-first 的 OpenAPI 3.0 文档生成能力。通过链式 Builder 描述 API 端点，Registry 收集所有端点并构建完整的 OpenAPI Spec，最终通过 ServeJSON/ServeYAML 以 HTTP Handler 形式输出文档。Schema 从 Go struct 自动反射生成，支持 json、validate、description 三种 struct tag。

## 功能特性

- Code-first：无需手写 YAML/JSON，从 Go 代码自动生成 OpenAPI 3.0.3 文档
- 链式 Builder：GET/POST/PUT/DELETE/PATCH 五种方法，流畅构建 Operation
- 反射 Schema：从 Go struct 自动生成 JSON Schema，支持嵌套、数组、指针
- Struct Tag 映射：json、validate、description 三种 tag 映射到 OpenAPI Schema
- HTTP Handler：ServeJSON/ServeYAML 直接挂载到路由

## Schema 生成规则

`SchemaFrom(v any) *Schema` 通过反射将 Go 值转换为 JSON Schema。

### Go 类型映射

| Go 类型 | Schema Type | Schema Format |
|---------|-------------|---------------|
| `string` | `string` | - |
| `bool` | `boolean` | - |
| `int/int8/.../uint64` | `integer` | - |
| `float32/float64` | `number` | - |
| `time.Time` | `string` | `date-time` |
| `slice/array` | `array` | - |
| `map` | `object` | - |
| `struct` | `object` | 递归解析各字段 |

### Struct Tag 映射

| Tag | 说明 | 示例 |
|-----|------|------|
| `json` | 字段名和 omitempty | `json:"name,omitempty"` |
| `validate` | 校验约束 → Schema 属性 | `validate:"required,min=1,max=100"` |
| `description` | 字段描述 | `description:"用户名"` |

**validate tag 解析规则：**

| validate 值 | Schema 效果 |
|-------------|-------------|
| `required` | 加入父级 `required` 数组（json tag 无 omitempty 时） |
| `min=N` | 设置 `minimum` |
| `max=N` | 设置 `maximum` |

## API

### Builder 方法

通过顶层函数创建 Builder，链式配置后调用 `Build()` 生成 Operation。

| 函数/方法 | 说明 |
|-----------|------|
| `GET(path) *Builder` | 创建 GET 操作 |
| `POST(path) *Builder` | 创建 POST 操作 |
| `PUT(path) *Builder` | 创建 PUT 操作 |
| `DELETE(path) *Builder` | 创建 DELETE 操作 |
| `PATCH(path) *Builder` | 创建 PATCH 操作 |
| `.Summary(string) *Builder` | 设置摘要 |
| `.Description(string) *Builder` | 设置描述 |
| `.Tags(...string) *Builder` | 设置标签 |
| `.OperationID(string) *Builder` | 设置操作 ID |
| `.Deprecated(bool) *Builder` | 标记已废弃 |
| `.Request(any) *Builder` | 设置请求体类型（反射生成 Schema） |
| `.Response(any) *Builder` | 设置响应类型（反射生成 Schema） |
| `.Errors(...any) *Builder` | 设置错误类型 |
| `.Build() *Operation` | 构建 Operation |

### Registry

| 函数/方法 | 说明 |
|-----------|------|
| `NewRegistry(opts ...RegistryOption) *Registry` | 创建 Registry |
| `.Add(ops ...*Operation)` | 注册 API 端点 |
| `.Build() *Spec` | 构建完整的 OpenAPI Spec |
| `.ServeJSON() http.Handler` | 返回输出 JSON 文档的 Handler |
| `.ServeYAML() http.Handler` | 返回输出 YAML 文档的 Handler |

### RegistryOption

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `WithInfo(title, version, description)` | `version="0.0.1"` | 设置文档标题、版本和描述 |
| `WithServer(url string, desc ...string)` | - | 添加服务器地址 |

## 使用示例

```go
import "github.com/Tsukikage7/servex/v2/openapi"

// 定义请求/响应类型
type CreateUserReq struct {
    Name  string `json:"name" validate:"required,min=1,max=50" description:"用户名"`
    Email string `json:"email" validate:"required" description:"邮箱"`
    Age   int    `json:"age" validate:"min=0,max=200" description:"年龄"`
}

type UserResp struct {
    ID    string `json:"id" description:"用户 ID"`
    Name  string `json:"name" description:"用户名"`
    Email string `json:"email" description:"邮箱"`
}

// 创建 Registry
reg := openapi.NewRegistry(
    openapi.WithInfo("My API", "1.0.0", "示例 API 文档"),
    openapi.WithServer("https://api.example.com", "生产环境"),
)

// 注册端点
reg.Add(
    openapi.POST("/users").
        Summary("创建用户").
        Tags("用户管理").
        OperationID("createUser").
        Request(CreateUserReq{}).
        Response(UserResp{}).
        Build(),

    openapi.GET("/users/{id}").
        Summary("获取用户").
        Tags("用户管理").
        OperationID("getUser").
        Response(UserResp{}).
        Build(),
)

// 挂载到路由
http.Handle("/openapi.json", reg.ServeJSON())
http.Handle("/openapi.yaml", reg.ServeYAML())
```
