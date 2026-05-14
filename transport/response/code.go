package response

import (
	"net/http"

	"github.com/Tsukikage7/servex/v2/errors"
	"google.golang.org/grpc/codes"
)

// Code 业务错误码.
type Code struct {
	Num     int         // 数字错误码
	Message string      // 默认错误消息（不可用 i18n 时的回退）
	Key     string      // i18n 消息键（可选，设置后由 LocalizedMessage 翻译）
	Kind    errors.Kind // 业务错误语义，用于推导 HTTP/gRPC 映射
	http    int         // 非标准 HTTP 映射覆盖值
}

// Error 实现 error 接口.
func (c Code) Error() string {
	return c.Message
}

// WithMessage 创建带自定义消息的错误码副本.
func (c Code) WithMessage(msg string) Code {
	c.Message = msg
	return c
}

// WithHTTPStatus 创建覆盖 HTTP 状态码的错误码副本.
//
// 常规自定义错误码优先使用 NewCodeWithKind，由 Kind 推导 HTTP/gRPC 映射；
// 只有需要非标准 HTTP 映射时再显式覆盖.
func (c Code) WithHTTPStatus(status int) Code {
	c.http = status
	return c
}

// HTTPStatus 返回错误码对应的 HTTP 状态码。
func (c Code) HTTPStatus() int {
	if c.http != 0 {
		return c.http
	}
	if c.Num == CodeSuccess.Num {
		return http.StatusOK
	}
	return c.Kind.HTTPStatus()
}

// GRPCCode 返回错误码对应的 gRPC 状态码。
func (c Code) GRPCCode() codes.Code {
	if c.Num == CodeSuccess.Num {
		return codes.OK
	}
	return c.Kind.GRPCCode()
}

// Is 判断是否为指定错误码，兼容 errors.Is.
func (c Code) Is(target error) bool {
	t, ok := target.(Code)
	if !ok {
		return false
	}
	return c.Num == t.Num
}

// ToError 将预定义错误码转为 *errors.Error（统一错误类型）.
//
// 推荐用法：
//
//	return response.CodeNotFound.ToError()
//	return response.CodeInvalidParam.ToError().WithMeta("field", "email")
func (c Code) ToError() *errors.Error {
	return errors.New(c.Num, c.Key, c.Message).
		WithKind(c.Kind)
}

// 预定义错误码.
//
// 错误码规范：
//   - 0: 成功
//   - 1xxxx: 通用错误
//   - 2xxxx: 认证/授权错误
//   - 3xxxx: 请求参数错误
//   - 4xxxx: 资源错误
//   - 5xxxx: 服务器内部错误
//   - 6xxxx: 外部服务错误
var (
	// CodeSuccess 成功.
	CodeSuccess = Code{Num: 0, Message: "成功", Key: "success"}

	// CodeUnknown 未知错误.
	CodeUnknown = Code{Num: 10000, Message: "未知错误", Key: "error.unknown", Kind: errors.KindUnknown}
	// CodeCanceled 请求已取消.
	CodeCanceled = Code{Num: 10001, Message: "请求已取消", Key: "error.canceled", Kind: errors.KindCanceled}
	// CodeTimeout 请求超时.
	CodeTimeout = Code{Num: 10002, Message: "请求超时", Key: "error.timeout", Kind: errors.KindDeadlineExceeded}

	// CodeUnauthorized 未授权.
	CodeUnauthorized = Code{Num: 20001, Message: "未授权", Key: "error.unauthorized", Kind: errors.KindUnauthenticated}
	// CodeForbidden 禁止访问.
	CodeForbidden = Code{Num: 20002, Message: "禁止访问", Key: "error.forbidden", Kind: errors.KindPermissionDenied}
	// CodeTokenExpired 令牌已过期.
	CodeTokenExpired = Code{Num: 20003, Message: "令牌已过期", Key: "error.token_expired", Kind: errors.KindUnauthenticated}
	// CodeTokenInvalid 令牌无效.
	CodeTokenInvalid = Code{Num: 20004, Message: "令牌无效", Key: "error.token_invalid", Kind: errors.KindUnauthenticated}

	// CodeInvalidParam 参数无效.
	CodeInvalidParam = Code{Num: 30001, Message: "参数无效", Key: "error.invalid_param", Kind: errors.KindInvalidArgument}
	// CodeMissingParam 缺少必需参数.
	CodeMissingParam = Code{Num: 30002, Message: "缺少必需参数", Key: "error.missing_param", Kind: errors.KindInvalidArgument}
	// CodeValidationFailed 参数验证失败.
	CodeValidationFailed = Code{Num: 30003, Message: "参数验证失败", Key: "error.validation", Kind: errors.KindInvalidArgument}

	// CodeNotFound 资源不存在.
	CodeNotFound = Code{Num: 40001, Message: "资源不存在", Key: "error.not_found", Kind: errors.KindNotFound}
	// CodeAlreadyExists 资源已存在.
	CodeAlreadyExists = Code{Num: 40002, Message: "资源已存在", Key: "error.already_exists", Kind: errors.KindConflict}
	// CodeConflict 资源冲突.
	CodeConflict = Code{Num: 40003, Message: "资源冲突", Key: "error.conflict", Kind: errors.KindConflict}
	// CodeResourceExhausted 资源耗尽.
	CodeResourceExhausted = Code{Num: 40004, Message: "资源耗尽", Key: "error.exhausted", Kind: errors.KindResourceExhausted}

	// CodeInternal 服务器内部错误.
	CodeInternal = Code{Num: 50001, Message: "服务器内部错误", Key: "error.internal", Kind: errors.KindInternal}
	// CodeNotImplemented 功能未实现.
	CodeNotImplemented = Code{Num: 50002, Message: "功能未实现", Key: "error.not_implemented", Kind: errors.KindNotImplemented}
	// CodeDatabaseError 数据库错误.
	CodeDatabaseError = Code{Num: 50003, Message: "数据库错误", Key: "error.database", Kind: errors.KindInternal}

	// CodeServiceUnavailable 服务不可用.
	CodeServiceUnavailable = Code{Num: 60001, Message: "服务不可用", Key: "error.unavailable", Kind: errors.KindUnavailable}
	// CodeUpstreamError 上游服务错误.
	CodeUpstreamError = Code{Num: 60002, Message: "上游服务错误", Key: "error.upstream", Kind: errors.KindUnavailable, http: http.StatusBadGateway}
)

// NewCodeWithKind 创建带业务语义的自定义错误码.
//
// 推荐用于新业务错误码：HTTP/gRPC 映射由 Kind 统一推导，避免调用方手写
// status code 和 gRPC code。
func NewCodeWithKind(num int, key, message string, kind errors.Kind) Code {
	return Code{
		Num:     num,
		Message: message,
		Key:     key,
		Kind:    kind,
	}
}
