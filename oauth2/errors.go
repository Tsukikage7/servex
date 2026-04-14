package oauth2

import (
	"net/http"

	"github.com/Tsukikage7/servex/v2/errors"
	"google.golang.org/grpc/codes"
)

// ErrInvalidState 表示 state 参数无效.
var ErrInvalidState = errors.New(80001, "oauth2.invalid_state", "state 无效").WithHTTP(http.StatusBadRequest).WithGRPC(codes.InvalidArgument)

// ErrExchangeFailed 表示 code 换取 token 失败.
var ErrExchangeFailed = errors.New(80002, "oauth2.exchange_failed", "code 换取 token 失败").WithHTTP(http.StatusBadGateway).WithGRPC(codes.Unavailable)

// ErrRefreshFailed 表示刷新 token 失败.
var ErrRefreshFailed = errors.New(80003, "oauth2.refresh_failed", "刷新 token 失败").WithHTTP(http.StatusBadGateway).WithGRPC(codes.Unavailable)

// ErrUserInfoFailed 表示获取用户信息失败.
var ErrUserInfoFailed = errors.New(80004, "oauth2.userinfo_failed", "获取用户信息失败").WithHTTP(http.StatusBadGateway).WithGRPC(codes.Unavailable)

// ErrInvalidCode 表示 code 为空.
var ErrInvalidCode = errors.New(80005, "oauth2.invalid_code", "code 为空").WithHTTP(http.StatusBadRequest).WithGRPC(codes.InvalidArgument)

// ErrInvalidToken 表示 token 为空.
var ErrInvalidToken = errors.New(80006, "oauth2.invalid_token", "token 为空").WithHTTP(http.StatusUnauthorized).WithGRPC(codes.Unauthenticated)
