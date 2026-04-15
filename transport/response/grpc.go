package response

import (
	"context"
	"encoding/json"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// grpcPayload 是嵌入 gRPC status message 的 JSON 载荷，用于跨 gRPC 边界保留细粒度业务 Code。
type grpcPayload struct {
	Num        int    `json:"num"`
	HTTPStatus int    `json:"http"`
	Key        string `json:"key,omitempty"`
	Message    string `json:"msg"`
}

// GRPCStatus 将错误转换为 gRPC Status.
//
// 如果是业务错误，使用对应的 gRPC 状态码；
// 否则返回 Internal 状态.
//
// 为了在 gRPC-gateway 场景下保留细粒度业务 Code（避免反向映射丢失），
// message 字段会序列化为 JSON，携带 Num/HTTPStatus/Key 等信息。
// FromGRPCStatus 会优先从 JSON 恢复，确保 30002/30003 等不被还原为 30001。
func GRPCStatus(err error) *status.Status {
	if err == nil {
		return status.New(codes.OK, "")
	}

	code := ExtractCode(err)
	message := ExtractMessage(err)

	payload := grpcPayload{
		Num:        code.Num,
		HTTPStatus: code.HTTPStatus,
		Key:        code.Key,
		Message:    message,
	}
	if b, jsonErr := json.Marshal(payload); jsonErr == nil {
		return status.New(code.GRPCCode, string(b))
	}
	return status.New(code.GRPCCode, message)
}

// GRPCError 将错误转换为 gRPC error.
//
// 返回的 error 可直接作为 gRPC 方法的返回值.
func GRPCError(err error) error {
	return GRPCStatus(err).Err()
}

// FromGRPCStatus 从 gRPC Status 提取 Code.
//
// 优先从 message JSON（由 GRPCStatus 序列化）中恢复完整的细粒度业务 Code；
// 若 message 不是 servex 格式，则回退到 gRPC code 映射。
func FromGRPCStatus(s *status.Status) Code {
	// 优先尝试从 JSON message 恢复（保留细粒度业务 Code，如 30002/30003）
	var p grpcPayload
	if json.Unmarshal([]byte(s.Message()), &p) == nil && p.Num != 0 {
		return Code{
			Num:        p.Num,
			HTTPStatus: p.HTTPStatus,
			GRPCCode:   s.Code(),
			Key:        p.Key,
			Message:    p.Message,
		}
	}

	// 回退：粗粒度 gRPC code 映射（兼容非 servex 来源的 gRPC 错误）
	switch s.Code() {
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
	code := FromGRPCStatus(status.New(c, ""))
	return code.HTTPStatus
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
func NewGRPCError(code Code) error {
	return status.Error(code.GRPCCode, code.Message)
}

// NewGRPCErrorWithMessage 创建带自定义消息的 gRPC 错误.
func NewGRPCErrorWithMessage(code Code, message string) error {
	return status.Error(code.GRPCCode, message)
}

// GRPCInterceptorErrorHandler gRPC 拦截器错误处理.
//
// 将业务错误转换为带有正确状态码的 gRPC 错误.
// 可用于 gRPC 拦截器中统一处理错误.
func GRPCInterceptorErrorHandler(err error) error {
	if err == nil {
		return nil
	}
	return GRPCError(err)
}

// UnaryServerInterceptor 返回 gRPC 一元服务器拦截器.
//
// 自动将业务错误转换为正确的 gRPC 状态码.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		resp, err := handler(ctx, req)
		if err != nil {
			return nil, GRPCError(err)
		}
		return resp, nil
	}
}

// StreamServerInterceptor 返回 gRPC 流服务器拦截器.
//
// 自动将业务错误转换为正确的 gRPC 状态码.
func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		err := handler(srv, ss)
		if err != nil {
			return GRPCError(err)
		}
		return nil
	}
}
