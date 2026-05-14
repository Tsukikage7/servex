# validation

封装 [go-playground/validator/v10](https://github.com/go-playground/validator)，提供带 i18n 错误消息的输入校验框架。

## 特性

- 封装 `go-playground/validator/v10`，开箱即用
- 内置中文 / 英文错误消息（`{field}` / `{param}` 占位符）
- 支持自定义错误消息模板
- 结构体校验与单字段校验
- 自动使用 `json` tag 作为字段名（友好的 API 错误响应）
- 支持注册自定义校验规则

## 快速开始

```go
import "github.com/Tsukikage7/servex/v2/validation"

v := validation.New() // 默认中文消息

type CreateUserReq struct {
    Name  string `json:"name"  validate:"required"`
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age"   validate:"gte=1,lte=120"`
}

if err := v.Validate(&CreateUserReq{}); err != nil {
    // err.(*validation.ValidationError).Errors 包含每个字段的错误
    fmt.Println(err) // validation failed: name (required), email (required), age (gte)
}
```

## 配置选项

```go
v := validation.New(
    validation.WithLocale("en"),              // 切换英文消息
    validation.WithTagName("binding"),        // 兼容 gin binding tag
    validation.WithMessages(map[string]string{
        "required": "{field}为必填项",        // 覆盖内置消息
        "mobile":   "{field}手机号格式不正确", // 自定义规则消息
    }),
)
```

## 注册自定义校验规则

```go
_ = v.RegisterValidation("mobile", func(fl validator.FieldLevel) bool {
    matched, _ := regexp.MatchString(`^1[3-9]\d{9}$`, fl.Field().String())
    return matched
})
```

## 单字段校验

```go
err := v.ValidateField("not-an-email", "required,email")
```

## 错误类型

```go
type ValidationError struct {
    Errors []FieldError `json:"errors"`
}

type FieldError struct {
    Field   string `json:"field"`
    Tag     string `json:"tag"`
    Value   any    `json:"value,omitempty"`
    Message string `json:"message"`
}
```

## 独立使用 ParseErrors

```go
// 将 go-playground/validator 原始错误转换为 ValidationError
ve := validation.ParseErrors(err, map[string]string{
    "required": "{field} is required",
})
```

## 内置消息支持的校验标签

`required` / `email` / `min` / `max` / `gte` / `lte` / `len` / `oneof` / `url` / `uuid`
