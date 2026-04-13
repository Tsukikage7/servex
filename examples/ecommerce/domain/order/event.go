package order

import "github.com/Tsukikage7/servex/v2/domain"

const (
	// EventOrderPlaced 订单下单事件名.
	EventOrderPlaced = "order.placed"
	// EventOrderCancelled 订单取消事件名.
	EventOrderCancelled = "order.cancelled"
	// EventOrderShipped 订单发货事件名.
	EventOrderShipped = "order.shipped"
	// EventOrderCompleted 订单完成事件名.
	EventOrderCompleted = "order.completed"
)

// OrderPlacedEvent 订单下单事件.
type OrderPlacedEvent struct {
	domain.BaseEvent
	OrderID     uint64
	UserID      uint64
	TotalAmount float64
}

// NewOrderPlacedEvent 创建订单下单事件.
func NewOrderPlacedEvent(orderID, userID uint64, totalAmount float64) *OrderPlacedEvent {
	return &OrderPlacedEvent{
		BaseEvent:   domain.NewBaseEvent(EventOrderPlaced),
		OrderID:     orderID,
		UserID:      userID,
		TotalAmount: totalAmount,
	}
}

// OrderCancelledEvent 订单取消事件.
type OrderCancelledEvent struct {
	domain.BaseEvent
	OrderID uint64
}

// NewOrderCancelledEvent 创建订单取消事件.
func NewOrderCancelledEvent(orderID uint64) *OrderCancelledEvent {
	return &OrderCancelledEvent{
		BaseEvent: domain.NewBaseEvent(EventOrderCancelled),
		OrderID:   orderID,
	}
}

// OrderShippedEvent 订单发货事件.
type OrderShippedEvent struct {
	domain.BaseEvent
	OrderID uint64
}

// NewOrderShippedEvent 创建订单发货事件.
func NewOrderShippedEvent(orderID uint64) *OrderShippedEvent {
	return &OrderShippedEvent{
		BaseEvent: domain.NewBaseEvent(EventOrderShipped),
		OrderID:   orderID,
	}
}

// OrderCompletedEvent 订单完成事件.
type OrderCompletedEvent struct {
	domain.BaseEvent
	OrderID uint64
}

// NewOrderCompletedEvent 创建订单完成事件.
func NewOrderCompletedEvent(orderID uint64) *OrderCompletedEvent {
	return &OrderCompletedEvent{
		BaseEvent: domain.NewBaseEvent(EventOrderCompleted),
		OrderID:   orderID,
	}
}
