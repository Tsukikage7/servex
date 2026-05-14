# servex 输入校验

## validation — 参数校验框架

```go
import "github.com/Tsukikage7/servex/v2/validation"

// 创建校验器（默认中文消息）
v := validation.New()

// 英文消息
v = validation.New(validation.WithLocale("en"))

// 自定义消息模板（支持 {field} 和 {param} 占位符）
v = validation.New(validation.WithMessages(map[string]string{
    "required": "{field}为必填项",
    "mobile":   "{field}手机号格式不正确",
}))

// 兼容 gin binding tag
v = validation.New(validation.WithTagName("binding"))
```

**校验结构体：**

```go
type CreateUserReq struct {
    Name  string `json:"name"  validate:"required"`
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age"   validate:"gte=1,lte=120"`
    Role  string `json:"role"  validate:"oneof=admin editor viewer"`
}

if err := v.Validate(&CreateUserReq{}); err != nil {
    ve := err.(*validation.ValidationError)
    for _, fe := range ve.Errors {
        fmt.Printf("字段: %s, 标签: %s, 消息: %s\n", fe.Field, fe.Tag, fe.Message)
    }
    // 直接返回 JSON 错误响应
    w.WriteHeader(http.StatusBadRequest)
    json.NewEncoder(w).Encode(ve)
}
```

**校验单个字段：**

```go
err := v.ValidateField("not-an-email", "required,email")
```

**注册自定义校验规则：**

```go
import "github.com/go-playground/validator/v10"

_ = v.RegisterValidation("mobile", func(fl validator.FieldLevel) bool {
    matched, _ := regexp.MatchString(`^1[3-9]\d{9}$`, fl.Field().String())
    return matched
})

// 然后在结构体中使用
type Req struct {
    Phone string `json:"phone" validate:"required,mobile"`
}
```

**错误类型：**

```go
type ValidationError struct {
    Errors []FieldError `json:"errors"`
}

type FieldError struct {
    Field   string `json:"field"`   // 来自 json tag
    Tag     string `json:"tag"`     // 校验标签名
    Value   any    `json:"value,omitempty"`
    Message string `json:"message"` // 人类可读的错误消息
}

// ValidationError.Error() 返回简洁描述
// "validation failed: name (required), email (email)"
```

**独立使用 ParseErrors（适合已有 validator 项目集成）：**

```go
err := validator.New().Struct(obj)
ve := validation.ParseErrors(err, map[string]string{
    "required": "{field} is required",
})
```

**内置支持的校验标签：**
`required` / `email` / `min` / `max` / `gte` / `lte` / `len` / `oneof` / `url` / `uuid`

底层基于 `go-playground/validator/v10`，支持所有原生标签，自定义消息仅对上述内置标签生效，其他标签使用默认格式 `{field}校验失败: {tag}`。
