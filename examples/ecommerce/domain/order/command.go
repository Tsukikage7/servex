package order

// OrderItemDTO 订单项数据传输对象.
type OrderItemDTO struct {
	ProductID   uint64  `json:"product_id" validate:"required"`
	ProductName string  `json:"product_name" validate:"required"`
	Quantity    int     `json:"quantity" validate:"required,min=1"`
	UnitPrice   float64 `json:"unit_price" validate:"required,gt=0"`
}

// PlaceOrderCommand 下单命令.
type PlaceOrderCommand struct {
	UserID uint64         `json:"user_id" validate:"required"`
	Items  []OrderItemDTO `json:"items" validate:"required,min=1,dive"`
}

// CancelOrderCommand 取消订单命令.
type CancelOrderCommand struct {
	ID uint64 `json:"-"`
}

// ShipOrderCommand 发货命令.
type ShipOrderCommand struct {
	ID uint64 `json:"-"`
}

// CompleteOrderCommand 完成订单命令.
type CompleteOrderCommand struct {
	ID uint64 `json:"-"`
}
