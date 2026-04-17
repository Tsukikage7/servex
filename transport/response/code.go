package response

import (
	"net/http"

	servexerr "github.com/Tsukikage7/servex/v2/errors"
	"google.golang.org/grpc/codes"
)

// Code 业务错误码.
type Code struct {
	Num        int        // 数字错误码
	Message    string     // 默认错误消息（不可用 i18n 时的回退）
	HTTPStatus int        // 对应的 HTTP 状态码
	GRPCCode   codes.Code // 对应的 gRPC 状态码
	Key        string     // i18n 消息键（可选，设置后由 LocalizedMessage 翻译）
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
func (c Code) ToError() *servexerr.Error {
	return servexerr.New(c.Num, c.Key, c.Message).
		WithHTTP(c.HTTPStatus).
		WithGRPC(c.GRPCCode)
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
	CodeSuccess = Code{Num: 0, Message: "成功", HTTPStatus: http.StatusOK, GRPCCode: codes.OK, Key: "success"}

	// CodeUnknown 未知错误.
	CodeUnknown = Code{Num: 10000, Message: "未知错误", HTTPStatus: http.StatusInternalServerError, GRPCCode: codes.Unknown, Key: "error.unknown"}
	// CodeCanceled 请求已取消.
	CodeCanceled = Code{Num: 10001, Message: "请求已取消", HTTPStatus: http.StatusRequestTimeout, GRPCCode: codes.Canceled, Key: "error.canceled"}
	// CodeTimeout 请求超时.
	CodeTimeout = Code{Num: 10002, Message: "请求超时", HTTPStatus: http.StatusGatewayTimeout, GRPCCode: codes.DeadlineExceeded, Key: "error.timeout"}

	// CodeUnauthorized 未授权.
	CodeUnauthorized = Code{Num: 20001, Message: "未授权", HTTPStatus: http.StatusUnauthorized, GRPCCode: codes.Unauthenticated, Key: "error.unauthorized"}
	// CodeForbidden 禁止访问.
	CodeForbidden = Code{Num: 20002, Message: "禁止访问", HTTPStatus: http.StatusForbidden, GRPCCode: codes.PermissionDenied, Key: "error.forbidden"}
	// CodeTokenExpired 令牌已过期.
	CodeTokenExpired = Code{Num: 20003, Message: "令牌已过期", HTTPStatus: http.StatusUnauthorized, GRPCCode: codes.Unauthenticated, Key: "error.token_expired"}
	// CodeTokenInvalid 令牌无效.
	CodeTokenInvalid = Code{Num: 20004, Message: "令牌无效", HTTPStatus: http.StatusUnauthorized, GRPCCode: codes.Unauthenticated, Key: "error.token_invalid"}

	// CodeInvalidParam 参数无效.
	CodeInvalidParam = Code{Num: 30001, Message: "参数无效", HTTPStatus: http.StatusBadRequest, GRPCCode: codes.InvalidArgument, Key: "error.invalid_param"}
	// CodeMissingParam 缺少必需参数.
	CodeMissingParam = Code{Num: 30002, Message: "缺少必需参数", HTTPStatus: http.StatusBadRequest, GRPCCode: codes.InvalidArgument, Key: "error.missing_param"}
	// CodeValidationFailed 参数验证失败.
	CodeValidationFailed = Code{Num: 30003, Message: "参数验证失败", HTTPStatus: http.StatusBadRequest, GRPCCode: codes.InvalidArgument, Key: "error.validation"}

	// CodeNotFound 资源不存在.
	CodeNotFound = Code{Num: 40001, Message: "资源不存在", HTTPStatus: http.StatusNotFound, GRPCCode: codes.NotFound, Key: "error.not_found"}
	// CodeAlreadyExists 资源已存在.
	CodeAlreadyExists = Code{Num: 40002, Message: "资源已存在", HTTPStatus: http.StatusConflict, GRPCCode: codes.AlreadyExists, Key: "error.already_exists"}
	// CodeConflict 资源冲突.
	CodeConflict = Code{Num: 40003, Message: "资源冲突", HTTPStatus: http.StatusConflict, GRPCCode: codes.Aborted, Key: "error.conflict"}
	// CodeResourceExhausted 资源耗尽.
	CodeResourceExhausted = Code{Num: 40004, Message: "资源耗尽", HTTPStatus: http.StatusTooManyRequests, GRPCCode: codes.ResourceExhausted, Key: "error.exhausted"}

	// CodeInternal 服务器内部错误.
	CodeInternal = Code{Num: 50001, Message: "服务器内部错误", HTTPStatus: http.StatusInternalServerError, GRPCCode: codes.Internal, Key: "error.internal"}
	// CodeNotImplemented 功能未实现.
	CodeNotImplemented = Code{Num: 50002, Message: "功能未实现", HTTPStatus: http.StatusNotImplemented, GRPCCode: codes.Unimplemented, Key: "error.not_implemented"}
	// CodeDatabaseError 数据库错误.
	CodeDatabaseError = Code{Num: 50003, Message: "数据库错误", HTTPStatus: http.StatusInternalServerError, GRPCCode: codes.Internal, Key: "error.database"}

	// CodeServiceUnavailable 服务不可用.
	CodeServiceUnavailable = Code{Num: 60001, Message: "服务不可用", HTTPStatus: http.StatusServiceUnavailable, GRPCCode: codes.Unavailable, Key: "error.unavailable"}
	// CodeUpstreamError 上游服务错误.
	CodeUpstreamError = Code{Num: 60002, Message: "上游服务错误", HTTPStatus: http.StatusBadGateway, GRPCCode: codes.Unavailable, Key: "error.upstream"}
)

// NewCode 创建自定义错误码.
func NewCode(num int, message string, httpStatus int, grpcCode codes.Code) Code {
	return Code{
		Num:        num,
		Message:    message,
		HTTPStatus: httpStatus,
		GRPCCode:   grpcCode,
	}
}
