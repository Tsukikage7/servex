package email

import (
	"net/http"

	"github.com/Tsukikage7/servex/v2/errors"
	"google.golang.org/grpc/codes"
)

// ErrEmptySMTPHost SMTP host 不能为空.
// ErrEmptyFromAddr 发件人地址不能为空.
var (
	ErrEmptySMTPHost = errors.New(70101, "notify.email.empty_smtp_host", "SMTP host 不能为空").WithHTTP(http.StatusBadRequest).WithGRPC(codes.InvalidArgument)
	ErrEmptyFromAddr = errors.New(70102, "notify.email.empty_from_addr", "发件人地址不能为空").WithHTTP(http.StatusBadRequest).WithGRPC(codes.InvalidArgument)
)
