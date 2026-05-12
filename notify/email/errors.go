package email

import (
	"github.com/Tsukikage7/servex/v2/errors"
)

// ErrEmptySMTPHost SMTP host 不能为空.
// ErrEmptyFromAddr 发件人地址不能为空.
var (
	ErrEmptySMTPHost = errors.NewWithKind(70101, "notify.email.empty_smtp_host", "SMTP host 不能为空", errors.KindInvalidArgument)
	ErrEmptyFromAddr = errors.NewWithKind(70102, "notify.email.empty_from_addr", "发件人地址不能为空", errors.KindInvalidArgument)
)
