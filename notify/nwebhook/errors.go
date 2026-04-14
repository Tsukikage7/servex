package nwebhook

import (
	"net/http"

	"github.com/Tsukikage7/servex/v2/errors"
	"google.golang.org/grpc/codes"
)

// ErrDeliveryFailed 投递失败.
var ErrDeliveryFailed = errors.New(70411, "notify.nwebhook.delivery_failed", "投递失败").WithHTTP(http.StatusBadGateway).WithGRPC(codes.Unavailable)
