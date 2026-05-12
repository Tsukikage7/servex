package jwt

import "github.com/Tsukikage7/servex/v2/errors"

var (
	// ErrTokenInvalid 令牌无效.
	ErrTokenInvalid = errors.NewWithKind(20101, "JWT_TOKEN_INVALID", "令牌无效或已过期", errors.KindUnauthenticated)

	// ErrTokenRevoked 令牌已撤销.
	ErrTokenRevoked = errors.NewWithKind(20102, "JWT_TOKEN_REVOKED", "令牌已撤销", errors.KindUnauthenticated)

	// ErrTokenEmpty 令牌为空.
	ErrTokenEmpty = errors.NewWithKind(20103, "JWT_TOKEN_EMPTY", "令牌不能为空", errors.KindUnauthenticated)

	// ErrTokenNotFound 未找到令牌.
	ErrTokenNotFound = errors.NewWithKind(20104, "JWT_TOKEN_NOT_FOUND", "未找到认证令牌", errors.KindUnauthenticated)

	// ErrSigningMethod 签名方法无效.
	ErrSigningMethod = errors.NewWithKind(20105, "JWT_SIGNING_METHOD", "无效的签名方法", errors.KindUnauthenticated)

	// ErrClaimsInvalid Claims 无效.
	ErrClaimsInvalid = errors.NewWithKind(20106, "JWT_CLAIMS_INVALID", "无效的 Claims", errors.KindUnauthenticated)

	// ErrRefreshExpired 刷新窗口已过期.
	ErrRefreshExpired = errors.NewWithKind(20107, "JWT_REFRESH_EXPIRED", "令牌已超出刷新窗口", errors.KindUnauthenticated)

	// ErrSigningKeyMissing 未配置签名密钥.
	ErrSigningKeyMissing = errors.NewWithKind(20108, "JWT_SIGNING_KEY_MISSING", "未配置签名密钥", errors.KindInternal)

	// ErrTokenStoreQuery 令牌存储查询失败.
	ErrTokenStoreQuery = errors.NewWithKind(20109, "JWT_TOKEN_STORE_QUERY", "令牌存储查询失败", errors.KindInternal)

	// ErrTokenStoreDelete 令牌存储删除失败.
	ErrTokenStoreDelete = errors.NewWithKind(20110, "JWT_TOKEN_STORE_DELETE", "令牌存储删除失败", errors.KindInternal)

	// ErrTokenStoreRevoke 令牌撤销标记设置失败.
	ErrTokenStoreRevoke = errors.NewWithKind(20111, "JWT_TOKEN_STORE_REVOKE", "令牌撤销标记设置失败", errors.KindInternal)
)
