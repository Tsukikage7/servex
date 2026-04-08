package order

import "time"

// GetOrderQuery 查询单个订单.
type GetOrderQuery struct {
	ID uint64
}

// ListOrdersQuery 分页查询订单列表.
type ListOrdersQuery struct {
	UserID uint64 `json:"user_id"`
	Offset int    `json:"offset"`
	Limit  int    `json:"limit"`
}

// OrderItemView 订单项视图对象.
type OrderItemView struct {
	ProductID   uint64  `json:"product_id"`
	ProductName string  `json:"product_name"`
	Quantity    int     `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	Subtotal    float64 `json:"subtotal"`
}

// OrderView 订单视图对象（返回给外部调用方）.
type OrderView struct {
	ID          uint64          `json:"id"`
	UserID      uint64          `json:"user_id"`
	Status      string          `json:"status"`
	Items       []OrderItemView `json:"items"`
	TotalAmount float64         `json:"total_amount"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// ToView 将聚合转换为视图对象.
func ToView(o *Order) *OrderView {
	items := make([]OrderItemView, 0, len(o.Items()))
	for _, item := range o.Items() {
		items = append(items, OrderItemView{
			ProductID:   item.ProductID(),
			ProductName: item.ProductName(),
			Quantity:    item.Quantity(),
			UnitPrice:   item.UnitPrice(),
			Subtotal:    item.Subtotal(),
		})
	}
	return &OrderView{
		ID:          o.ID(),
		UserID:      o.UserID(),
		Status:      o.Status().String(),
		Items:       items,
		TotalAmount: o.TotalAmount(),
		CreatedAt:   o.CreatedAt(),
		UpdatedAt:   o.UpdatedAt(),
	}
}
