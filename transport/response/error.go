package response

import (
	"errors"
	"fmt"

	servexerr "github.com/Tsukikage7/servex/v2/errors"
)

// BusinessError 业务错误.
//
// Deprecated: 请使用 Code.ToError() 或 errors.New() 创建错误.
// 此类型保留用于向后兼容，新代码应使用 servex/errors.Error.
type BusinessError struct {
	Code    Code   // 错误码
	Message string // 自定义错误消息（可选）
	Cause   error  // 原始错误（可选）
}

// Error 实现 error 接口.
func (e *BusinessError) Error() string {
	msg := e.Message
	if msg == "" {
		msg = e.Code.Message
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", msg, e.Cause)
	}
	return msg
}

// Unwrap 返回原始错误.
func (e *BusinessError) Unwrap() error {
	return e.Cause
}

// GetCode 获取错误码.
func (e *BusinessError) GetCode() Code {
	return e.Code
}

// GetMessage 获取错误消息.
func (e *BusinessError) GetMessage() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Code.Message
}

// NewError 创建业务错误.
//
// Deprecated: 请使用 Code.ToError() 代替.
func NewError(code Code) *BusinessError {
	return &BusinessError{
		Code: code,
	}
}

// NewErrorWithMessage 创建带自定义消息的业务错误.
//
// Deprecated: 请使用 Code.ToError().WithMessage(msg) 代替.
func NewErrorWithMessage(code Code, message string) *BusinessError {
	return &BusinessError{
		Code:    code,
		Message: message,
	}
}

// NewErrorWithCause 创建带原始错误的业务错误.
//
// Deprecated: 请使用 Code.ToError().WithCause(err) 代替.
func NewErrorWithCause(code Code, cause error) *BusinessError {
	return &BusinessError{
		Code:  code,
		Cause: cause,
	}
}

// NewErrorFull 创建完整的业务错误.
//
// Deprecated: 请使用 Code.ToError().WithMessage(msg).WithCause(err) 代替.
func NewErrorFull(code Code, message string, cause error) *BusinessError {
	return &BusinessError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// Wrap 包装错误为业务错误.
//
// Deprecated: 请使用 Code.ToError().WithCause(err) 代替.
func Wrap(code Code, err error) *BusinessError {
	return &BusinessError{
		Code:  code,
		Cause: err,
	}
}

// WrapWithMessage 包装错误为带消息的业务错误.
//
// Deprecated: 请使用 Code.ToError().WithMessage(msg).WithCause(err) 代替.
func WrapWithMessage(code Code, message string, err error) *BusinessError {
	return &BusinessError{
		Code:    code,
		Message: message,
		Cause:   err,
	}
}

// IsBusinessError 判断是否为业务错误（含 *BusinessError 和 *errors.Error）.
func IsBusinessError(err error) bool {
	if _, ok := errors.AsType[*BusinessError](err); ok {
		return true
	}
	_, ok := servexerr.FromError(err)
	return ok
}

// AsBusinessError 将错误转换为业务错误.
//
// 如果不是业务错误，返回 nil.
func AsBusinessError(err error) *BusinessError {
	bizErr, _ := errors.AsType[*BusinessError](err)
	return bizErr
}

// ExtractCode 从错误中提取错误码.
//
// 如果是业务错误，返回对应的错误码；
// 否则返回 CodeInternal.
func ExtractCode(err error) Code {
	if err == nil {
		return CodeSuccess
	}

	if bizErr, ok := errors.AsType[*BusinessError](err); ok {
		return bizErr.Code
	}

	// 检查是否直接是 Code 类型
	if code, ok := errors.AsType[Code](err); ok {
		return code
	}

	// 桥接 servex/errors.Error → response.Code
	if srvErr, ok := servexerr.FromError(err); ok {
		return Code{
			Num:        srvErr.Code,
			Message:    srvErr.Message,
			HTTPStatus: srvErr.HTTP,
			GRPCCode:   srvErr.GRPC,
			Key:        srvErr.Key,
		}
	}

	return CodeInternal
}

// ExtractMetadata 从错误中提取元数据（仅 servex/errors.Error 携带 metadata）.
//
// 返回 nil 表示错误无元数据或不是 servex/errors.Error.
func ExtractMetadata(err error) map[string]string {
	if err == nil {
		return nil
	}
	if srvErr, ok := servexerr.FromError(err); ok {
		return srvErr.Metadata
	}
	return nil
}

// ExtractMessage 从错误中提取错误消息.
//
// 对于内部错误（5xxxx、6xxxx），返回通用消息，避免暴露敏感信息.
func ExtractMessage(err error) string {
	if err == nil {
		return CodeSuccess.Message
	}

	// 先检查 BusinessError/Code（业务明确声明的，可信）
	if bizErr, ok := errors.AsType[*BusinessError](err); ok {
		if isInternalCode(bizErr.Code.Num) {
			return bizErr.Code.Message
		}
		return bizErr.GetMessage()
	}
	if code, ok := errors.AsType[Code](err); ok {
		return code.Message
	}

	// 再检查 servex/errors.Error（可能携带 cause 等敏感信息）
	if srvErr, ok := servexerr.FromError(err); ok {
		if isInternalCode(srvErr.Code) {
			return CodeInternal.Message
		}
		return srvErr.Message
	}

	return CodeInternal.Message
}

// isInternalCode 判断是否为内部/外部服务错误（应掩码 Message）.
// 规范：5xxxx=服务器内部，6xxxx=外部服务；业务码 >= 70000 不掩码.
func isInternalCode(code int) bool {
	return code >= 50000 && code < 70000
}

// ExtractMessageUnsafe 从错误中提取完整错误消息（包含敏感信息）.
//
// 仅用于日志记录，不应返回给客户端.
func ExtractMessageUnsafe(err error) string {
	if err == nil {
		return CodeSuccess.Message
	}

	if bizErr, ok := errors.AsType[*BusinessError](err); ok {
		if bizErr.Cause != nil {
			return fmt.Sprintf("%s: %v", bizErr.GetMessage(), bizErr.Cause)
		}
		return bizErr.GetMessage()
	}

	return err.Error()
}
