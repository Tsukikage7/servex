package config

import (
	"errors"
	"fmt"
)

var (
	// ErrNilConfig 配置为空.
	ErrNilConfig = errors.New("配置为空")

	// ErrFileNotFound 配置文件不存在.
	ErrFileNotFound = errors.New("配置文件不存在")

	// ErrInvalidType 不支持的配置文件类型.
	ErrInvalidType = errors.New("不支持的配置文件类型")

	// ErrReadConfig 读取配置失败.
	ErrReadConfig = errors.New("读取配置失败")

	// ErrUnmarshal 解析配置失败.
	ErrUnmarshal = errors.New("解析配置失败")

	// ErrValidation 配置验证失败.
	ErrValidation = errors.New("配置验证失败")

	// ErrSourceLoad 加载配置源失败.
	ErrSourceLoad = errors.New("加载配置源失败")

	// ErrSourceWatch 监听配置变更失败.
	ErrSourceWatch = errors.New("监听配置变更失败")

	// ErrSourceClosed 配置源已关闭.
	ErrSourceClosed = errors.New("配置源已关闭")
)

// ConfigFieldError 配置字段错误.
type ConfigFieldError struct {
	Field    string // 字段路径 (如 "database.host")
	Source   string // 来源 (如 "file:config.yaml")
	Message  string // 错误描述
	Expected string // 期望类型/值
	Actual   string // 实际值
	Err      error  // 内部错误，用于 Unwrap
}

// Error 返回格式化的错误信息.
func (e *ConfigFieldError) Error() string {
	msg := fmt.Sprintf("config: 字段 %q (来源: %s): %s", e.Field, e.Source, e.Message)
	if e.Expected != "" || e.Actual != "" {
		msg += fmt.Sprintf(" (期望: %s, 实际: %s)", e.Expected, e.Actual)
	}
	return msg
}

// Unwrap 返回内部错误以支持 errors.Is 匹配.
func (e *ConfigFieldError) Unwrap() error {
	if e.Err != nil {
		return e.Err
	}
	return ErrValidation
}

// NewFieldError 创建配置字段错误.
func NewFieldError(field, source, message string) *ConfigFieldError {
	return &ConfigFieldError{
		Field:   field,
		Source:  source,
		Message: message,
	}
}

// NewFieldTypeError 创建类型不匹配的配置字段错误.
func NewFieldTypeError(field, source, expected, actual string) *ConfigFieldError {
	return &ConfigFieldError{
		Field:    field,
		Source:   source,
		Message:  "类型不匹配",
		Expected: expected,
		Actual:   actual,
	}
}
