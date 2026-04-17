package response

import (
	"encoding/json"

	servexerr "github.com/Tsukikage7/servex/v2/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// legacyPayload 是 response 包旧版 GRPCStatus 嵌入的 JSON 格式.
// 仅用于 FromGRPCStatus 的向后兼容读取，新代码不再输出此格式.
type legacyPayload struct {
	Num        int    `json:"num"`
	HTTPStatus int    `json:"http"`
	Key        string `json:"key,omitempty"`
	Message    string `json:"msg"`
}

// GRPCStatus 将错误转换为 gRPC Status.
//
// 统一委托给 servex/errors.ToGRPCStatus，使用统一的 JSON 格式.
// 支持 *errors.Error 和 *BusinessError（通过 ExtractCode 桥接）.
func GRPCStatus(err error) *status.Status {
	if err == nil {
		return status.New(codes.OK, "")
	}

	// 优先检查是否已经是 *errors.Error，直接委托.
	if _, ok := servexerr.FromError(err); ok {
		return servexerr.ToGRPCStatus(err)
	}

	// BusinessError/Code/普通 error → 桥接为 *errors.Error 后委托.
	code := ExtractCode(err)
	message := ExtractMessage(err)
	bridged := servexerr.New(code.Num, code.Key, message).
		WithHTTP(code.HTTPStatus).
		WithGRPC(code.GRPCCode)

	// 尽量保留 metadata（仅 BusinessError 无 metadata，不影响）
	if md := ExtractMetadata(err); md != nil {
		for k, v := range md {
			bridged = bridged.WithMeta(k, v)
		}
	}

	return servexerr.ToGRPCStatus(bridged)
}

// GRPCError 将错误转换为 gRPC error.
func GRPCError(err error) error {
	return GRPCStatus(err).Err()
}

// FromGRPCStatus 从 gRPC Status 提取 Code.
//
// 优先使用 servex/errors.FromGRPCStatus 还原完整信息，
// 兼容旧版 response 格式（{"num":...,"http":...,"msg":"..."}），
// 最后回退到 gRPC code 映射.
func FromGRPCStatus(s *status.Status) Code {
	// 1. 尝试用 errors 包还原（统一格式 + ErrorInfo details）
	if restored := servexerr.FromGRPCStatus(s); restored != nil {
		httpStatus := restored.HTTP
		if httpStatus == 0 {
			httpStatus = grpcCodeToHTTPFallback(s.Code())
		}
		return Code{
			Num:        restored.Code,
			Message:    restored.Message,
			HTTPStatus: httpStatus,
			GRPCCode:   restored.GRPC,
			Key:        restored.Key,
		}
	}

	// 2. 兼容旧版 response 格式（过渡期）
	var p legacyPayload
	if json.Unmarshal([]byte(s.Message()), &p) == nil && p.Num != 0 {
		return Code{
			Num:        p.Num,
			HTTPStatus: p.HTTPStatus,
			GRPCCode:   s.Code(),
			Key:        p.Key,
			Message:    p.Message,
		}
	}

	// 3. 粗粒度 gRPC code 映射
	return grpcCodeFallback(s.Code())
}

// FromGRPCError 从 gRPC error 提取 Code.
func FromGRPCError(err error) Code {
	if err == nil {
		return CodeSuccess
	}
	s, ok := status.FromError(err)
	if !ok {
		return CodeInternal
	}
	return FromGRPCStatus(s)
}

// GRPCCodeToHTTP 将 gRPC 状态码转换为 HTTP 状态码.
func GRPCCodeToHTTP(c codes.Code) int {
	return grpcCodeToHTTPFallback(c)
}

// HTTPToGRPCCode 将 HTTP 状态码转换为 gRPC 状态码.
func HTTPToGRPCCode(httpStatus int) codes.Code {
	switch {
	case httpStatus >= 200 && httpStatus < 300:
		return codes.OK
	case httpStatus == 400:
		return codes.InvalidArgument
	case httpStatus == 401:
		return codes.Unauthenticated
	case httpStatus == 403:
		return codes.PermissionDenied
	case httpStatus == 404:
		return codes.NotFound
	case httpStatus == 408:
		return codes.DeadlineExceeded
	case httpStatus == 409:
		return codes.AlreadyExists
	case httpStatus == 429:
		return codes.ResourceExhausted
	case httpStatus == 499:
		return codes.Canceled
	case httpStatus == 500:
		return codes.Internal
	case httpStatus == 501:
		return codes.Unimplemented
	case httpStatus == 502, httpStatus == 503, httpStatus == 504:
		return codes.Unavailable
	default:
		return codes.Unknown
	}
}

// NewGRPCError 创建 gRPC 错误.
//
// Deprecated: 请使用 Code.ToError() 代替.
func NewGRPCError(code Code) error {
	return status.Error(code.GRPCCode, code.Message)
}

// NewGRPCErrorWithMessage 创建带自定义消息的 gRPC 错误.
//
// Deprecated: 请使用 Code.ToError().WithMessage(msg) 代替.
func NewGRPCErrorWithMessage(code Code, message string) error {
	return status.Error(code.GRPCCode, message)
}

// GRPCInterceptorErrorHandler gRPC 拦截器错误处理.
//
// Deprecated: 请使用 UnaryServerInterceptor() 代替.
func GRPCInterceptorErrorHandler(err error) error {
	if err == nil {
		return nil
	}
	return GRPCError(err)
}

// UnaryServerInterceptor 返回 gRPC 一元服务器拦截器.
//
// 统一委托给 servex/errors.UnaryServerInterceptor.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return servexerr.UnaryServerInterceptor()
}

// StreamServerInterceptor 返回 gRPC 流服务器拦截器.
//
// 统一委托给 servex/errors.StreamServerInterceptor.
func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return servexerr.StreamServerInterceptor()
}

// grpcCodeToHTTPFallback 根据 gRPC code 推断 HTTP status.
func grpcCodeToHTTPFallback(c codes.Code) int {
	switch c {
	case codes.OK:
		return 200
	case codes.InvalidArgument:
		return 400
	case codes.Unauthenticated:
		return 401
	case codes.PermissionDenied:
		return 403
	case codes.NotFound:
		return 404
	case codes.AlreadyExists:
		return 409
	case codes.ResourceExhausted:
		return 429
	case codes.Internal:
		return 500
	case codes.Unavailable:
		return 503
	default:
		return 500
	}
}

// grpcCodeFallback 粗粒度 gRPC code → Code 映射.
func grpcCodeFallback(c codes.Code) Code {
	switch c {
	case codes.OK:
		return CodeSuccess
	case codes.Canceled:
		return CodeCanceled
	case codes.Unknown:
		return CodeUnknown
	case codes.InvalidArgument:
		return CodeInvalidParam
	case codes.DeadlineExceeded:
		return CodeTimeout
	case codes.NotFound:
		return CodeNotFound
	case codes.AlreadyExists:
		return CodeAlreadyExists
	case codes.PermissionDenied:
		return CodeForbidden
	case codes.ResourceExhausted:
		return CodeResourceExhausted
	case codes.Aborted:
		return CodeConflict
	case codes.Unimplemented:
		return CodeNotImplemented
	case codes.Internal:
		return CodeInternal
	case codes.Unavailable:
		return CodeServiceUnavailable
	case codes.Unauthenticated:
		return CodeUnauthorized
	default:
		return CodeUnknown
	}
}
