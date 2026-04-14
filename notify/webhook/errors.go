package webhook

import (
	"net/http"

	"github.com/Tsukikage7/servex/v2/errors"
	"google.golang.org/grpc/codes"
)

// ErrNilSubscription 订阅为空.
// ErrNilEvent 事件为空.
// ErrEmptyURL 订阅 URL 为空.
// ErrInvalidSignature 签名验证失败.
// ErrEmptyBody 请求体为空.
// ErrNotFound 未找到.
// ErrDeliveryFailed 投递失败.
var (
	ErrNilSubscription  = errors.New(70401, "notify.webhook.nil_subscription", "subscription 为空").WithHTTP(http.StatusBadRequest).WithGRPC(codes.InvalidArgument)
	ErrNilEvent         = errors.New(70402, "notify.webhook.nil_event", "event 为空").WithHTTP(http.StatusBadRequest).WithGRPC(codes.InvalidArgument)
	ErrEmptyURL         = errors.New(70403, "notify.webhook.empty_url", "subscription URL 为空").WithHTTP(http.StatusBadRequest).WithGRPC(codes.InvalidArgument)
	ErrInvalidSignature = errors.New(70404, "notify.webhook.invalid_signature", "签名验证失败").WithHTTP(http.StatusUnauthorized).WithGRPC(codes.Unauthenticated)
	ErrEmptyBody        = errors.New(70405, "notify.webhook.empty_body", "请求体为空").WithHTTP(http.StatusBadRequest).WithGRPC(codes.InvalidArgument)
	ErrNotFound         = errors.New(70406, "notify.webhook.not_found", "未找到").WithHTTP(http.StatusNotFound).WithGRPC(codes.NotFound)
	ErrDeliveryFailed   = errors.New(70407, "notify.webhook.delivery_failed", "投递失败").WithHTTP(http.StatusBadGateway).WithGRPC(codes.Unavailable)
)
