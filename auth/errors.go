package auth

import "github.com/Tsukikage7/servex/v2/errors"

var (
	// ErrUnauthenticated 未认证错误.
	ErrUnauthenticated = errors.NewWithKind(20001, "AUTH_UNAUTHENTICATED", "未认证", errors.KindUnauthenticated)

	// ErrForbidden 无权限错误.
	ErrForbidden = errors.NewWithKind(20002, "AUTH_FORBIDDEN", "无权限", errors.KindPermissionDenied)

	// ErrInvalidCredentials 无效凭据错误.
	ErrInvalidCredentials = errors.NewWithKind(20003, "AUTH_INVALID_CREDENTIALS", "无效凭据", errors.KindUnauthenticated)

	// ErrCredentialsExpired 凭据已过期错误.
	ErrCredentialsExpired = errors.NewWithKind(20004, "AUTH_CREDENTIALS_EXPIRED", "凭据已过期", errors.KindUnauthenticated)

	// ErrCredentialsNotFound 凭据未找到错误.
	ErrCredentialsNotFound = errors.NewWithKind(20005, "AUTH_CREDENTIALS_NOT_FOUND", "凭据未找到", errors.KindUnauthenticated)
)

// IsUnauthenticated 检查是否为未认证错误.
func IsUnauthenticated(err error) bool {
	return errors.CodeIs(err, ErrUnauthenticated)
}

// IsForbidden 检查是否为无权限错误.
func IsForbidden(err error) bool {
	return errors.CodeIs(err, ErrForbidden)
}
