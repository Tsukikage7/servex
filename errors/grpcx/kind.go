package grpcx

import (
	"google.golang.org/grpc/codes"

	serrors "github.com/Tsukikage7/servex/v2/errors"
)

// CodeForKind 返回 kind 对应的默认 gRPC 状态码。
func CodeForKind(kind serrors.Kind) codes.Code {
	switch kind {
	case serrors.KindCanceled:
		return codes.Canceled
	case serrors.KindNotFound:
		return codes.NotFound
	case serrors.KindConflict:
		return codes.AlreadyExists
	case serrors.KindInvalidArgument:
		return codes.InvalidArgument
	case serrors.KindPermissionDenied:
		return codes.PermissionDenied
	case serrors.KindUnauthenticated:
		return codes.Unauthenticated
	case serrors.KindFailedPrecondition:
		return codes.FailedPrecondition
	case serrors.KindUnavailable:
		return codes.Unavailable
	case serrors.KindDeadlineExceeded:
		return codes.DeadlineExceeded
	case serrors.KindResourceExhausted:
		return codes.ResourceExhausted
	case serrors.KindNotImplemented:
		return codes.Unimplemented
	case serrors.KindUnknown:
		return codes.Unknown
	default:
		return codes.Internal
	}
}
