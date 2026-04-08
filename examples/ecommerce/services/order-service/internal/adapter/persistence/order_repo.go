package persistence

import (
	"context"
	"errors"

	"gorm.io/gorm"

	domainOrder "github.com/Tsukikage7/servex/examples/ecommerce/domain/order"
)

// OrderRepository 基于 GORM 的订单仓储实现.
type OrderRepository struct {
	db *gorm.DB
}

// NewOrderRepository 创建订单仓储.
func NewOrderRepository(db *gorm.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

// Create 创建订单.
func (r *OrderRepository) Create(ctx context.Context, order *domainOrder.Order) error {
	po, err := FromAggregate(order)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(po).Error
}

// GetByID 根据 ID 查询订单.
func (r *OrderRepository) GetByID(ctx context.Context, id uint64) (*domainOrder.Order, error) {
	var po OrderPO
	if err := r.db.WithContext(ctx).First(&po, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domainOrder.ErrNotFound
		}
		return nil, err
	}
	return po.ToAggregate()
}

// Update 更新订单.
func (r *OrderRepository) Update(ctx context.Context, order *domainOrder.Order) error {
	po, err := FromAggregate(order)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Save(po).Error
}

// FindByUserID 查询指定用户的订单列表.
func (r *OrderRepository) FindByUserID(ctx context.Context, userID uint64, offset, limit int) ([]*domainOrder.Order, int64, error) {
	return r.List(ctx, domainOrder.Filter{
		UserID: userID,
		Offset: offset,
		Limit:  limit,
	})
}

// List 按条件分页查询订单列表.
func (r *OrderRepository) List(ctx context.Context, filter domainOrder.Filter) ([]*domainOrder.Order, int64, error) {
	query := r.db.WithContext(ctx).Model(&OrderPO{})

	if filter.UserID > 0 {
		query = query.Where("user_id = ?", filter.UserID)
	}
	if filter.Status != nil {
		query = query.Where("status = ?", int(*filter.Status))
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if filter.Limit <= 0 {
		filter.Limit = 20
	}

	var pos []OrderPO
	if err := query.Offset(filter.Offset).Limit(filter.Limit).Order("created_at DESC").Find(&pos).Error; err != nil {
		return nil, 0, err
	}

	orders := make([]*domainOrder.Order, 0, len(pos))
	for _, po := range pos {
		o, err := po.ToAggregate()
		if err != nil {
			return nil, 0, err
		}
		orders = append(orders, o)
	}
	return orders, total, nil
}
