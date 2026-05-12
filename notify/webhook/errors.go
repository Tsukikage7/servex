package webhook

import (
	"github.com/Tsukikage7/servex/v2/errors"
)

// ErrNilSubscription 订阅为空.
// ErrNilEvent 事件为空.
// ErrEmptyURL 订阅 URL 为空.
// ErrInvalidSignature 签名验证失败.
// ErrEmptyBody 请求体为空.
// ErrNotFound 未找到.
// ErrDeliveryFailed 投递失败.
var (
	ErrNilSubscription  = errors.NewWithKind(70401, "notify.webhook.nil_subscription", "subscription 为空", errors.KindInvalidArgument)
	ErrNilEvent         = errors.NewWithKind(70402, "notify.webhook.nil_event", "event 为空", errors.KindInvalidArgument)
	ErrEmptyURL         = errors.NewWithKind(70403, "notify.webhook.empty_url", "subscription URL 为空", errors.KindInvalidArgument)
	ErrInvalidSignature = errors.NewWithKind(70404, "notify.webhook.invalid_signature", "签名验证失败", errors.KindUnauthenticated)
	ErrEmptyBody        = errors.NewWithKind(70405, "notify.webhook.empty_body", "请求体为空", errors.KindInvalidArgument)
	ErrNotFound         = errors.NewWithKind(70406, "notify.webhook.not_found", "未找到", errors.KindNotFound)
	ErrDeliveryFailed   = errors.NewWithKind(70407, "notify.webhook.delivery_failed", "投递失败", errors.KindUnavailable)
)
