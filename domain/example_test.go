package domain_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/domain"
)

// OrderCreated 是一个示例领域事件.
type OrderCreated struct {
	domain.BaseEvent
	OrderID string
}

func ExampleNewAggregateRoot() {
	root := domain.NewAggregateRoot[string]("order-001")
	fmt.Println(root.ID())
	// Output: order-001
}

func ExampleAggregateRoot_RaiseEvent() {
	root := domain.NewAggregateRoot[string]("order-001")

	// 发起领域事件.
	event := OrderCreated{
		BaseEvent: domain.NewBaseEvent("OrderCreated"),
		OrderID:   "order-001",
	}
	root.RaiseEvent(event)

	fmt.Println(len(root.DomainEvents()))
	fmt.Println(root.DomainEvents()[0].EventName())
	// Output:
	// 1
	// OrderCreated
}

func ExampleAggregateRoot_ClearDomainEvents() {
	root := domain.NewAggregateRoot[string]("order-001")

	root.RaiseEvent(OrderCreated{
		BaseEvent: domain.NewBaseEvent("OrderCreated"),
		OrderID:   "order-001",
	})
	fmt.Println("before clear:", len(root.DomainEvents()))

	root.ClearDomainEvents()
	fmt.Println("after clear:", len(root.DomainEvents()))
	// Output:
	// before clear: 1
	// after clear: 0
}
