// Package persistence 订单持久化适配器.
package persistence

import (
	"encoding/json"
	"time"

	domainOrder "github.com/Tsukikage7/servex/v2/examples/ecommerce/domain/order"
)

// OrderPO 订单持久化对象.
type OrderPO struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement:false"`
	UserID      uint64    `gorm:"not null;index"`
	Status      int       `gorm:"not null;default:0"`
	Items       string    `gorm:"type:text;not null"` // JSON 序列化
	TotalAmount float64   `gorm:"type:decimal(10,2);not null"`
	CreatedAt   time.Time `gorm:"not null"`
	UpdatedAt   time.Time `gorm:"not null"`
}

// TableName 指定表名.
func (OrderPO) TableName() string { return "orders" }

// OrderItemJSON 订单项 JSON 结构.
type OrderItemJSON struct {
	ProductID   uint64  `json:"product_id"`
	ProductName string  `json:"product_name"`
	Quantity    int     `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
}

// ToAggregate 将持久化对象转换为领域聚合.
func (po *OrderPO) ToAggregate() (*domainOrder.Order, error) {
	var itemsJSON []OrderItemJSON
	if err := json.Unmarshal([]byte(po.Items), &itemsJSON); err != nil {
		return nil, err
	}

	items := make([]*domainOrder.OrderItem, 0, len(itemsJSON))
	for _, ij := range itemsJSON {
		items = append(items, domainOrder.NewOrderItem(ij.ProductID, ij.ProductName, ij.Quantity, ij.UnitPrice))
	}

	return domainOrder.ReconstructOrder(
		po.ID,
		po.UserID,
		domainOrder.OrderStatus(po.Status),
		items,
		po.TotalAmount,
		po.CreatedAt,
		po.UpdatedAt,
	), nil
}

// FromAggregate 将领域聚合转换为持久化对象.
func FromAggregate(o *domainOrder.Order) (*OrderPO, error) {
	itemsJSON := make([]OrderItemJSON, 0, len(o.Items()))
	for _, item := range o.Items() {
		itemsJSON = append(itemsJSON, OrderItemJSON{
			ProductID:   item.ProductID(),
			ProductName: item.ProductName(),
			Quantity:    item.Quantity(),
			UnitPrice:   item.UnitPrice(),
		})
	}

	data, err := json.Marshal(itemsJSON)
	if err != nil {
		return nil, err
	}

	return &OrderPO{
		ID:          o.ID(),
		UserID:      o.UserID(),
		Status:      int(o.Status()),
		Items:       string(data),
		TotalAmount: o.TotalAmount(),
		CreatedAt:   o.CreatedAt(),
		UpdatedAt:   o.UpdatedAt(),
	}, nil
}
