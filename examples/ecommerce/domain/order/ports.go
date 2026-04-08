package order

import "context"

// UserProvider 用户服务防腐层，订单领域通过此接口获取用户信息.
type UserProvider interface {
	// UserExists 检查用户是否存在.
	UserExists(ctx context.Context, userID uint64) (bool, error)
}
