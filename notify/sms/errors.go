package sms

import (
	"github.com/Tsukikage7/servex/v2/errors"
)

// ErrNilProvider provider 不能为空.
// ErrPartialSendFailed 部分发送失败.
var (
	ErrNilProvider       = errors.NewWithKind(70201, "notify.sms.nil_provider", "provider 不能为空", errors.KindInvalidArgument)
	ErrPartialSendFailed = errors.NewWithKind(70202, "notify.sms.partial_send_failed", "部分发送失败", errors.KindInternal)
)
