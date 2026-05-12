package oauth2

import (
	"github.com/Tsukikage7/servex/v2/errors"
)

// ErrInvalidState 表示 state 参数无效.
var ErrInvalidState = errors.NewWithKind(80001, "oauth2.invalid_state", "state 无效", errors.KindInvalidArgument)

// ErrExchangeFailed 表示 code 换取 token 失败.
var ErrExchangeFailed = errors.NewWithKind(80002, "oauth2.exchange_failed", "code 换取 token 失败", errors.KindUnavailable)

// ErrRefreshFailed 表示刷新 token 失败.
var ErrRefreshFailed = errors.NewWithKind(80003, "oauth2.refresh_failed", "刷新 token 失败", errors.KindUnavailable)

// ErrUserInfoFailed 表示获取用户信息失败.
var ErrUserInfoFailed = errors.NewWithKind(80004, "oauth2.userinfo_failed", "获取用户信息失败", errors.KindUnavailable)

// ErrInvalidCode 表示 code 为空.
var ErrInvalidCode = errors.NewWithKind(80005, "oauth2.invalid_code", "code 为空", errors.KindInvalidArgument)

// ErrInvalidToken 表示 token 为空.
var ErrInvalidToken = errors.NewWithKind(80006, "oauth2.invalid_token", "token 为空", errors.KindUnauthenticated)
