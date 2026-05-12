// Package errors 提供统一的业务错误定义，支持 HTTP 和 gRPC 状态码映射.
package errors

import (
	stderrors "errors"
	"fmt"
)

// Error 统一业务错误.
type Error struct {
	Code     int               `json:"code"`
	Key      string            `json:"key"`
	Message  string            `json:"message"`
	Kind     Kind              `json:"-"`
	Metadata map[string]string `json:"metadata,omitzero"`
	cause    error
}

// New 创建错误定义.
func New(code int, key, message string) *Error {
	return &Error{
		Code:    code,
		Key:     key,
		Message: message,
	}
}

// NewWithKind 创建带业务语义映射的错误定义。
func NewWithKind(code int, key, message string, kind Kind) *Error {
	return New(code, key, message).WithKind(kind)
}

// Error 实现 error 接口.
func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("[%d] %s: %s: %v", e.Code, e.Key, e.Message, e.cause)
	}
	return fmt.Sprintf("[%d] %s: %s", e.Code, e.Key, e.Message)
}

// Unwrap 支持 errors.Is / errors.As 链式解包.
func (e *Error) Unwrap() error {
	return e.cause
}

// Is 支持 errors.Is 按 Code+Key 比较.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	return e.Code == t.Code && e.Key == t.Key
}

// WithKind 绑定业务错误语义。
func (e *Error) WithKind(kind Kind) *Error {
	cp := e.clone()
	cp.Kind = kind
	return cp
}

// clone 返回浅拷贝，保护包级变量原始定义.
func (e *Error) clone() *Error {
	cp := *e
	if e.Metadata != nil {
		cp.Metadata = make(map[string]string, len(e.Metadata))
		for k, v := range e.Metadata {
			cp.Metadata[k] = v
		}
	}
	return &cp
}

// WithCause 包装底层错误，返回新实例.
func (e *Error) WithCause(err error) *Error {
	cp := e.clone()
	cp.cause = err
	return cp
}

// WithMeta 附加元数据，返回新实例.
func (e *Error) WithMeta(key, value string) *Error {
	cp := e.clone()
	if cp.Metadata == nil {
		cp.Metadata = make(map[string]string)
	}
	cp.Metadata[key] = value
	return cp
}

// WithMessage 覆盖消息，返回新实例.
func (e *Error) WithMessage(msg string) *Error {
	cp := e.clone()
	cp.Message = msg
	return cp
}

// FromError 从 error 中提取 *Error.
func FromError(err error) (*Error, bool) {
	if err == nil {
		return nil, false
	}
	e, ok := stderrors.AsType[*Error](err)
	return e, ok
}

// CodeIs 判断 err 是否包含与 target 相同 Code 的 *Error.
func CodeIs(err error, target *Error) bool {
	e, ok := FromError(err)
	if !ok {
		return false
	}
	return e.Code == target.Code
}
