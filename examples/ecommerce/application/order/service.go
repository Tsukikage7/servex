// Package order 订单应用服务.
package order

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Tsukikage7/servex/v2/domain"

	domainOrder "github.com/Tsukikage7/servex/v2/examples/ecommerce/domain/order"
)

// ErrUserNotFound 用户不存在.
var ErrUserNotFound = errors.New("order: 关联用户不存在")

// Service 订单应用服务.
type Service struct {
	repo         domainOrder.Repository
	eventBus     *domain.EventBus
	userProvider domainOrder.UserProvider
}

// NewService 创建订单应用服务.
func NewService(repo domainOrder.Repository, eventBus *domain.EventBus, userProvider domainOrder.UserProvider) *Service {
	return &Service{
		repo:         repo,
		eventBus:     eventBus,
		userProvider: userProvider,
	}
}

// PlaceOrder 下单.
func (s *Service) PlaceOrder(ctx context.Context, cmd domainOrder.PlaceOrderCommand) (*domainOrder.OrderView, error) {
	// 通过防腐层校验用户是否存在
	exists, err := s.userProvider.UserExists(ctx, cmd.UserID)
	if err != nil {
		return nil, fmt.Errorf("校验用户失败: %w", err)
	}
	if !exists {
		return nil, ErrUserNotFound
	}

	// 转换命令中的订单项
	items := make([]*domainOrder.OrderItem, 0, len(cmd.Items))
	for _, dto := range cmd.Items {
		items = append(items, domainOrder.NewOrderItem(dto.ProductID, dto.ProductName, dto.Quantity, dto.UnitPrice))
	}

	// 使用时间戳作为简易 ID 生成
	id := uint64(time.Now().UnixNano())

	order, err := domainOrder.Place(id, cmd.UserID, items)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Create(ctx, order); err != nil {
		return nil, fmt.Errorf("创建订单失败: %w", err)
	}

	// 分发领域事件
	if err := s.eventBus.Dispatch(ctx, order.DomainEvents(), order.ClearDomainEvents); err != nil {
		return nil, fmt.Errorf("分发领域事件失败: %w", err)
	}

	return domainOrder.ToView(order), nil
}

// GetByID 根据 ID 查询订单.
func (s *Service) GetByID(ctx context.Context, id uint64) (*domainOrder.OrderView, error) {
	order, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return domainOrder.ToView(order), nil
}

// CancelOrder 取消订单.
func (s *Service) CancelOrder(ctx context.Context, cmd domainOrder.CancelOrderCommand) (*domainOrder.OrderView, error) {
	order, err := s.repo.GetByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}

	if err := order.Cancel(); err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, order); err != nil {
		return nil, fmt.Errorf("更新订单失败: %w", err)
	}

	if err := s.eventBus.Dispatch(ctx, order.DomainEvents(), order.ClearDomainEvents); err != nil {
		return nil, fmt.Errorf("分发领域事件失败: %w", err)
	}

	return domainOrder.ToView(order), nil
}

// ShipOrder 发货.
func (s *Service) ShipOrder(ctx context.Context, cmd domainOrder.ShipOrderCommand) (*domainOrder.OrderView, error) {
	order, err := s.repo.GetByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}

	if err := order.Ship(); err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, order); err != nil {
		return nil, fmt.Errorf("更新订单失败: %w", err)
	}

	if err := s.eventBus.Dispatch(ctx, order.DomainEvents(), order.ClearDomainEvents); err != nil {
		return nil, fmt.Errorf("分发领域事件失败: %w", err)
	}

	return domainOrder.ToView(order), nil
}

// CompleteOrder 完成订单.
func (s *Service) CompleteOrder(ctx context.Context, cmd domainOrder.CompleteOrderCommand) (*domainOrder.OrderView, error) {
	order, err := s.repo.GetByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}

	if err := order.Complete(); err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, order); err != nil {
		return nil, fmt.Errorf("更新订单失败: %w", err)
	}

	if err := s.eventBus.Dispatch(ctx, order.DomainEvents(), order.ClearDomainEvents); err != nil {
		return nil, fmt.Errorf("分发领域事件失败: %w", err)
	}

	return domainOrder.ToView(order), nil
}

// ListByUserID 查询指定用户的订单列表.
func (s *Service) ListByUserID(ctx context.Context, query domainOrder.ListOrdersQuery) ([]*domainOrder.OrderView, int64, error) {
	orders, total, err := s.repo.FindByUserID(ctx, query.UserID, query.Offset, query.Limit)
	if err != nil {
		return nil, 0, err
	}

	views := make([]*domainOrder.OrderView, 0, len(orders))
	for _, o := range orders {
		views = append(views, domainOrder.ToView(o))
	}
	return views, total, nil
}
