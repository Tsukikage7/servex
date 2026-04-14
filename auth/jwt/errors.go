package jwt

import (
	"google.golang.org/grpc/codes"

	"github.com/Tsukikage7/servex/v2/errors"
)

var (
	// ErrTokenInvalid 令牌无效.
	ErrTokenInvalid = errors.New(20101, "JWT_TOKEN_INVALID", "令牌无效或已过期").
		WithHTTP(401).WithGRPC(codes.Unauthenticated)

	// ErrTokenRevoked 令牌已撤销.
	ErrTokenRevoked = errors.New(20102, "JWT_TOKEN_REVOKED", "令牌已撤销").
		WithHTTP(401).WithGRPC(codes.Unauthenticated)

	// ErrTokenEmpty 令牌为空.
	ErrTokenEmpty = errors.New(20103, "JWT_TOKEN_EMPTY", "令牌不能为空").
		WithHTTP(401).WithGRPC(codes.Unauthenticated)

	// ErrTokenNotFound 未找到令牌.
	ErrTokenNotFound = errors.New(20104, "JWT_TOKEN_NOT_FOUND", "未找到认证令牌").
		WithHTTP(401).WithGRPC(codes.Unauthenticated)

	// ErrSigningMethod 签名方法无效.
	ErrSigningMethod = errors.New(20105, "JWT_SIGNING_METHOD", "无效的签名方法").
		WithHTTP(401).WithGRPC(codes.Unauthenticated)

	// ErrClaimsInvalid Claims 无效.
	ErrClaimsInvalid = errors.New(20106, "JWT_CLAIMS_INVALID", "无效的 Claims").
		WithHTTP(401).WithGRPC(codes.Unauthenticated)

	// ErrRefreshExpired 刷新窗口已过期.
	ErrRefreshExpired = errors.New(20107, "JWT_REFRESH_EXPIRED", "令牌已超出刷新窗口").
		WithHTTP(401).WithGRPC(codes.Unauthenticated)

	// ErrSigningKeyMissing 未配置签名密钥.
	ErrSigningKeyMissing = errors.New(20108, "JWT_SIGNING_KEY_MISSING", "未配置签名密钥").
		WithHTTP(500).WithGRPC(codes.Internal)

	// ErrTokenStoreQuery 令牌存储查询失败.
	ErrTokenStoreQuery = errors.New(20109, "JWT_TOKEN_STORE_QUERY", "令牌存储查询失败").
		WithHTTP(500).WithGRPC(codes.Internal)

	// ErrTokenStoreDelete 令牌存储删除失败.
	ErrTokenStoreDelete = errors.New(20110, "JWT_TOKEN_STORE_DELETE", "令牌存储删除失败").
		WithHTTP(500).WithGRPC(codes.Internal)

	// ErrTokenStoreRevoke 令牌撤销标记设置失败.
	ErrTokenStoreRevoke = errors.New(20111, "JWT_TOKEN_STORE_REVOKE", "令牌撤销标记设置失败").
		WithHTTP(500).WithGRPC(codes.Internal)
)
