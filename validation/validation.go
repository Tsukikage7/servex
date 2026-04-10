// Package validation 封装 go-playground/validator，提供带 i18n 错误消息的输入校验框架.
// 特性：
//   - 封装 go-playground/validator/v10
//   - 内置中文/英文错误消息
//   - 支持自定义错误消息模板（{field}/{param} 占位符）
//   - 结构体校验 & 单字段校验
//   - 注册自定义校验规则
// 示例：
//	v := validation.New()
//	type User struct {
//	    Name  string `json:"name" validate:"required"`
//	    Email string `json:"email" validate:"required,email"`
//	}
//	if err := v.Validate(&User{}); err != nil {
//	    fmt.Println(err) // validation failed: name (required), email (required)
//	}
package validation

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

// 内置中文错误消息.
var defaultZhMessages = map[string]string{
	"required": "{field}不能为空",
	"email":    "{field}格式不正确",
	"min":      "{field}长度不能小于{param}",
	"max":      "{field}长度不能大于{param}",
	"gte":      "{field}不能小于{param}",
	"lte":      "{field}不能大于{param}",
	"len":      "{field}长度必须为{param}",
	"oneof":    "{field}必须是以下值之一: {param}",
	"url":      "{field}必须是有效的URL",
	"uuid":     "{field}必须是有效的UUID",
}

// 内置英文错误消息.
var defaultEnMessages = map[string]string{
	"required": "{field} is required",
	"email":    "{field} must be a valid email",
	"min":      "{field} must be at least {param}",
	"max":      "{field} must be at most {param}",
	"gte":      "{field} must be greater than or equal to {param}",
	"lte":      "{field} must be less than or equal to {param}",
	"len":      "{field} must have length {param}",
	"oneof":    "{field} must be one of: {param}",
	"url":      "{field} must be a valid URL",
	"uuid":     "{field} must be a valid UUID",
}

// Validatable 可由请求/配置对象实现以启用自动校验.
//
// Handle 等函数在解码后自动检查该接口并调用 Validate().
type Validatable interface {
	Validate() error
}

// Validator 校验器.
type Validator struct {
	validate *validator.Validate
	tagName  string
	messages map[string]string
	locale   string
}

// Option 校验器配置选项.
type Option func(*Validator)

// WithTagName 设置校验标签名，默认 "validate".
func WithTagName(tag string) Option {
	return func(v *Validator) {
		v.tagName = tag
	}
}

// WithMessages 自定义错误消息模板.
// 模板支持 {field} 和 {param} 占位符.
func WithMessages(msgs map[string]string) Option {
	return func(v *Validator) {
		for k, msg := range msgs {
			v.messages[k] = msg
		}
	}
}

// WithLocale 设置语言，支持 "zh"/"en"，默认 "zh".
func WithLocale(locale string) Option {
	return func(v *Validator) {
		v.locale = locale
	}
}

// New 创建校验器.
func New(opts ...Option) *Validator {
	v := &Validator{
		validate: validator.New(),
		tagName:  "validate",
		messages: make(map[string]string),
		locale:   "zh",
	}

	for _, opt := range opts {
		opt(v)
	}

	// 根据 locale 设置默认消息
	defaults := defaultZhMessages
	if v.locale == "en" {
		defaults = defaultEnMessages
	}
	for k, msg := range defaults {
		if _, ok := v.messages[k]; !ok {
			v.messages[k] = msg
		}
	}

	v.validate.SetTagName(v.tagName)

	// 使用 json tag 作为字段名
	v.validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := fld.Tag.Get("json")
		if name == "" || name == "-" {
			return fld.Name
		}
		// 取 json tag 逗号前的部分
		if idx := strings.Index(name, ","); idx != -1 {
			name = name[:idx]
		}
		return name
	})

	return v
}

// Validate 校验结构体.
func (v *Validator) Validate(obj any) error {
	err := v.validate.Struct(obj)
	if err == nil {
		return nil
	}
	return ParseErrors(err, v.messages)
}

// ValidateField 校验单个字段.
func (v *Validator) ValidateField(field any, tag string) error {
	err := v.validate.Var(field, tag)
	if err == nil {
		return nil
	}
	return ParseErrors(err, v.messages)
}

// RegisterValidation 注册自定义校验规则.
func (v *Validator) RegisterValidation(tag string, fn validator.Func) error {
	return v.validate.RegisterValidation(tag, fn)
}

// FieldError 校验错误.
type FieldError struct {
	Field   string `json:"field"`
	Tag     string `json:"tag"`
	Value   any    `json:"value,omitempty"`
	Message string `json:"message"`
}

// ValidationError 校验错误集合.
type ValidationError struct {
	Errors []FieldError `json:"errors"`
}

// Error 返回校验错误的描述.
func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Errors))
	for _, fe := range e.Errors {
		parts = append(parts, fmt.Sprintf("%s (%s)", fe.Field, fe.Tag))
	}
	return "validation failed: " + strings.Join(parts, ", ")
}

// ParseErrors 将 validator 的错误转换为 ValidationError.
// msgs 为可选的自定义错误消息模板.
func ParseErrors(err error, msgs ...map[string]string) *ValidationError {
	var messages map[string]string
	if len(msgs) > 0 && msgs[0] != nil {
		messages = msgs[0]
	} else {
		messages = defaultZhMessages
	}

	var fieldErrors []FieldError

	if ve, ok := err.(validator.ValidationErrors); ok {
		for _, fe := range ve {
			fieldErr := FieldError{
				Field: fe.Field(),
				Tag:   fe.Tag(),
				Value: fe.Value(),
			}

			// 生成消息
			if tpl, ok := messages[fe.Tag()]; ok {
				msg := strings.ReplaceAll(tpl, "{field}", fe.Field())
				msg = strings.ReplaceAll(msg, "{param}", fe.Param())
				fieldErr.Message = msg
			} else {
				fieldErr.Message = fmt.Sprintf("%s校验失败: %s", fe.Field(), fe.Tag())
			}

			fieldErrors = append(fieldErrors, fieldErr)
		}
	}

	return &ValidationError{Errors: fieldErrors}
}
