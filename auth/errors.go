package auth

import (
	"google.golang.org/grpc/codes"

	"github.com/Tsukikage7/servex/v2/errors"
)

var (
	// ErrUnauthenticated 未认证错误.
	ErrUnauthenticated = errors.New(20001, "AUTH_UNAUTHENTICATED", "未认证").
		WithHTTP(401).WithGRPC(codes.Unauthenticated)

	// ErrForbidden 无权限错误.
	ErrForbidden = errors.New(20002, "AUTH_FORBIDDEN", "无权限").
		WithHTTP(403).WithGRPC(codes.PermissionDenied)

	// ErrInvalidCredentials 无效凭据错误.
	ErrInvalidCredentials = errors.New(20003, "AUTH_INVALID_CREDENTIALS", "无效凭据").
		WithHTTP(401).WithGRPC(codes.Unauthenticated)

	// ErrCredentialsExpired 凭据已过期错误.
	ErrCredentialsExpired = errors.New(20004, "AUTH_CREDENTIALS_EXPIRED", "凭据已过期").
		WithHTTP(401).WithGRPC(codes.Unauthenticated)

	// ErrCredentialsNotFound 凭据未找到错误.
	ErrCredentialsNotFound = errors.New(20005, "AUTH_CREDENTIALS_NOT_FOUND", "凭据未找到").
		WithHTTP(401).WithGRPC(codes.Unauthenticated)
)

// IsUnauthenticated 检查是否为未认证错误.
func IsUnauthenticated(err error) bool {
	return errors.CodeIs(err, ErrUnauthenticated)
}

// IsForbidden 检查是否为无权限错误.
func IsForbidden(err error) bool {
	return errors.CodeIs(err, ErrForbidden)
}
