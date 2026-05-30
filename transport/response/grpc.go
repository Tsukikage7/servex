package response

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	errorsgrpcx "github.com/Tsukikage7/servex/v2/errors/grpcx"
)

// GRPCStatus 将错误转换为 gRPC Status.
//
// 统一委托给 servex/errors/grpcx.ToGRPCStatus，使用统一的 JSON 格式.
// 支持 *errors.Error 和 Code通过 ExtractCode 桥接.
func GRPCStatus(err error) *status.Status {
	if err == nil {
		return status.New(codes.OK, "")
	}

	// 通过 response.Code 归一化后桥接为 *errors.Error，确保内置 Code 只按 Kind 派生 gRPC code。
	code := ExtractCode(err)
	message := ExtractMessage(err)
	bridged := code.ToError().WithMessage(message)

	// 尽量保留 metadata.
	if md := ExtractMetadata(err); md != nil {
		for k, v := range md {
			bridged = bridged.WithMeta(k, v)
		}
	}

	return errorsgrpcx.ToGRPCStatus(bridged)
}

// GRPCError 将错误转换为 gRPC error.
func GRPCError(err error) error {
	return GRPCStatus(err).Err()
}

// FromGRPCStatus 从 gRPC Status 提取 Code.
//
// 优先使用 servex/errors/grpcx.FromGRPCStatus 还原完整信息，
// 最后回退到 gRPC code 映射.
func FromGRPCStatus(s *status.Status) Code {
	if restored := errorsgrpcx.FromGRPCStatus(s); restored != nil {
		code := Code{
			Num:     restored.Code,
			Message: restored.Message,
			Key:     restored.Key,
			Kind:    restored.Kind,
		}
		return normalizeCode(code)
	}

	if s == nil {
		return CodeSuccess
	}

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

// UnaryServerInterceptor 返回 gRPC 一元服务器拦截器.
//
// 统一委托给 servex/errors/grpcx.UnaryServerInterceptor.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return errorsgrpcx.UnaryServerInterceptor()
}

// StreamServerInterceptor 返回 gRPC 流服务器拦截器.
//
// 统一委托给 servex/errors.StreamServerInterceptor.
func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return errorsgrpcx.StreamServerInterceptor()
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
