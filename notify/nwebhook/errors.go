package nwebhook

import (
	"github.com/Tsukikage7/servex/v2/errors"
)

// ErrDeliveryFailed 投递失败.
var ErrDeliveryFailed = errors.NewWithKind(70411, "notify.nwebhook.delivery_failed", "投递失败", errors.KindUnavailable)
