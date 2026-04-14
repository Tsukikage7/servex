package push

import (
	"net/http"

	"github.com/Tsukikage7/servex/v2/errors"
	"google.golang.org/grpc/codes"
)

// ErrNilProvider provider 不能为空.
// ErrPartialSendFailed 部分发送失败.
var (
	ErrNilProvider      = errors.New(70301, "notify.push.nil_provider", "provider 不能为空").WithHTTP(http.StatusBadRequest).WithGRPC(codes.InvalidArgument)
	ErrPartialSendFailed = errors.New(70302, "notify.push.partial_send_failed", "部分发送失败").WithHTTP(http.StatusInternalServerError).WithGRPC(codes.Internal)
)
