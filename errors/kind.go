package errors

import "net/http"

// Kind 描述业务错误语义，用于统一映射 HTTP 与 gRPC 状态。
type Kind int

const (
	KindInternal Kind = iota
	KindUnknown
	KindCanceled
	KindNotFound
	KindConflict
	KindInvalidArgument
	KindPermissionDenied
	KindUnauthenticated
	KindFailedPrecondition
	KindUnavailable
	KindDeadlineExceeded
	KindResourceExhausted
	KindNotImplemented
)

// HTTPStatus 返回 kind 对应的默认 HTTP 状态码。
func (k Kind) HTTPStatus() int {
	switch k {
	case KindCanceled:
		return http.StatusRequestTimeout
	case KindNotFound:
		return http.StatusNotFound
	case KindConflict:
		return http.StatusConflict
	case KindInvalidArgument:
		return http.StatusBadRequest
	case KindPermissionDenied:
		return http.StatusForbidden
	case KindUnauthenticated:
		return http.StatusUnauthorized
	case KindFailedPrecondition:
		return http.StatusPreconditionFailed
	case KindUnavailable:
		return http.StatusServiceUnavailable
	case KindDeadlineExceeded:
		return http.StatusGatewayTimeout
	case KindResourceExhausted:
		return http.StatusTooManyRequests
	case KindNotImplemented:
		return http.StatusNotImplemented
	default:
		return http.StatusInternalServerError
	}
}
