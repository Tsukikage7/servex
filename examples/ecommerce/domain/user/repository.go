package user

import (
	"context"
	"net/http"

	"google.golang.org/grpc/codes"

	"github.com/Tsukikage7/servex/v2/errors"
)

// ErrNotFound 用户不存在.
var ErrNotFound = errors.New(40101, "user.not_found", "用户不存在").WithHTTP(http.StatusNotFound).WithGRPC(codes.NotFound)

// Filter 用户查询过滤条件.
type Filter struct {
	Username string
	Email    string
	Offset   int
	Limit    int
}

// Repository 用户仓储接口.
type Repository interface {
	// Create 创建用户.
	Create(ctx context.Context, user *User) error

	// GetByID 根据 ID 查询用户.
	GetByID(ctx context.Context, id uint64) (*User, error)

	// GetByEmail 根据邮箱查询用户.
	GetByEmail(ctx context.Context, email string) (*User, error)

	// Update 更新用户.
	Update(ctx context.Context, user *User) error

	// List 按条件分页查询用户列表.
	List(ctx context.Context, filter Filter) ([]*User, int64, error)
}
