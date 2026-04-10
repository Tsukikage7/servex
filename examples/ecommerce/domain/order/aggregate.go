// Package order 订单领域模型.
package order

import (
	"errors"
	"time"

	"github.com/Tsukikage7/servex/domain"
)

// OrderStatus 订单状态.
type OrderStatus int

const (
	// StatusPending 待处理.
	StatusPending OrderStatus = iota
	// StatusPaid 已支付.
	StatusPaid
	// StatusShipped 已发货.
	StatusShipped
	// StatusCompleted 已完成.
	StatusCompleted
	// StatusCancelled 已取消.
	StatusCancelled
)

// String 返回订单状态的字符串表示.
func (s OrderStatus) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusPaid:
		return "paid"
	case StatusShipped:
		return "shipped"
	case StatusCompleted:
		return "completed"
	case StatusCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

// 订单领域错误.
var (
	ErrEmptyItems       = errors.New("order: 订单项不能为空")
	ErrAlreadyCancelled = errors.New("order: 订单已取消")
	ErrAlreadyCompleted = errors.New("order: 订单已完成")
	ErrNotShippable     = errors.New("order: 当前状态不可发货")
	ErrNotCompletable   = errors.New("order: 当前状态不可完成")
)

// Order 订单聚合根.
type Order struct {
	domain.AggregateRoot[uint64]
	userID      uint64
	status      OrderStatus
	items       []*OrderItem
	totalAmount float64
	createdAt   time.Time
	updatedAt   time.Time
}

// Place 创建（下单）订单.
func Place(id, userID uint64, items []*OrderItem) (*Order, error) {
	if len(items) == 0 {
		return nil, ErrEmptyItems
	}

	var total float64
	for _, item := range items {
		total += item.Subtotal()
	}

	o := &Order{
		AggregateRoot: domain.NewAggregateRoot(id),
		userID:        userID,
		status:        StatusPending,
		items:         items,
		totalAmount:   total,
		createdAt:     time.Now(),
		updatedAt:     time.Now(),
	}
	o.RaiseEvent(NewOrderPlacedEvent(id, userID, total))
	return o, nil
}

// ReconstructOrder 从持久化数据重建订单聚合（不触发事件）.
func ReconstructOrder(id, userID uint64, status OrderStatus, items []*OrderItem, totalAmount float64, createdAt, updatedAt time.Time) *Order {
	return &Order{
		AggregateRoot: domain.NewAggregateRoot(id),
		userID:        userID,
		status:        status,
		items:         items,
		totalAmount:   totalAmount,
		createdAt:     createdAt,
		updatedAt:     updatedAt,
	}
}

// Cancel 取消订单.
func (o *Order) Cancel() error {
	if o.status == StatusCancelled {
		return ErrAlreadyCancelled
	}
	if o.status == StatusCompleted {
		return ErrAlreadyCompleted
	}
	o.status = StatusCancelled
	o.updatedAt = time.Now()
	o.RaiseEvent(NewOrderCancelledEvent(o.ID()))
	return nil
}

// Ship 发货.
func (o *Order) Ship() error {
	if o.status != StatusPending && o.status != StatusPaid {
		return ErrNotShippable
	}
	o.status = StatusShipped
	o.updatedAt = time.Now()
	o.RaiseEvent(NewOrderShippedEvent(o.ID()))
	return nil
}

// Complete 完成订单.
func (o *Order) Complete() error {
	if o.status != StatusShipped {
		return ErrNotCompletable
	}
	o.status = StatusCompleted
	o.updatedAt = time.Now()
	o.RaiseEvent(NewOrderCompletedEvent(o.ID()))
	return nil
}

// UserID 返回用户 ID.
func (o *Order) UserID() uint64 { return o.userID }

// Status 返回订单状态.
func (o *Order) Status() OrderStatus { return o.status }

// Items 返回订单项列表.
func (o *Order) Items() []*OrderItem { return o.items }

// TotalAmount 返回订单总金额.
func (o *Order) TotalAmount() float64 { return o.totalAmount }

// CreatedAt 返回创建时间.
func (o *Order) CreatedAt() time.Time { return o.createdAt }

// UpdatedAt 返回更新时间.
func (o *Order) UpdatedAt() time.Time { return o.updatedAt }
