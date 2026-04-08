package order

// OrderItem 订单项（值对象）.
type OrderItem struct {
	productID   uint64
	productName string
	quantity    int
	unitPrice   float64
}

// NewOrderItem 创建订单项.
func NewOrderItem(productID uint64, productName string, quantity int, unitPrice float64) *OrderItem {
	return &OrderItem{
		productID:   productID,
		productName: productName,
		quantity:    quantity,
		unitPrice:   unitPrice,
	}
}

// ProductID 返回商品 ID.
func (i *OrderItem) ProductID() uint64 { return i.productID }

// ProductName 返回商品名称.
func (i *OrderItem) ProductName() string { return i.productName }

// Quantity 返回数量.
func (i *OrderItem) Quantity() int { return i.quantity }

// UnitPrice 返回单价.
func (i *OrderItem) UnitPrice() float64 { return i.unitPrice }

// Subtotal 返回小计金额.
func (i *OrderItem) Subtotal() float64 { return float64(i.quantity) * i.unitPrice }
