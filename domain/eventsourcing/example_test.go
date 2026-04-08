package eventsourcing_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/domain/eventsourcing"
)

func ExampleNewBaseAggregate() {
	agg := eventsourcing.NewBaseAggregate("order-001", "Order")
	fmt.Println(agg.AggregateID())
	fmt.Println(agg.AggregateType())
	fmt.Println(agg.Version())
	// Output:
	// order-001
	// Order
	// 0
}

func ExampleEvent_TableName() {
	e := eventsourcing.Event{}
	fmt.Println(e.TableName())
	// Output:
	// events
}
