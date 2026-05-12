package order

import (
	"context"

	"github.com/Tsukikage7/servex/v2/errors"
)

// ErrNotFound 订单不存在.
var ErrNotFound = errors.NewWithKind(40010, "order.not_found", "订单不存在", errors.KindNotFound)

// Filter 订单查询过滤条件.
type Filter struct {
	UserID uint64
	Status *OrderStatus
	Offset int
	Limit  int
}

// Repository 订单仓储接口.
type Repository interface {
	// Create 创建订单.
	Create(ctx context.Context, order *Order) error

	// GetByID 根据 ID 查询订单.
	GetByID(ctx context.Context, id uint64) (*Order, error)

	// Update 更新订单.
	Update(ctx context.Context, order *Order) error

	// FindByUserID 查询指定用户的订单列表.
	FindByUserID(ctx context.Context, userID uint64, offset, limit int) ([]*Order, int64, error)

	// List 按条件分页查询订单列表.
	List(ctx context.Context, filter Filter) ([]*Order, int64, error)
}
