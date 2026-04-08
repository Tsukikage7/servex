package saga_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/domain/saga"
)

func ExampleData() {
	data := saga.NewData()
	data.Set("order_id", "ORD-001")
	data.Set("amount", 100)

	fmt.Println(data.GetString("order_id"))
	fmt.Println(data.GetInt("amount"))
	fmt.Println(data.GetString("missing"))
	// Output:
	// ORD-001
	// 100
	//
}

func ExampleStep() {
	step := saga.Step{
		Name:   "reserve-inventory",
		Action: nil,
	}
	fmt.Println(step.Name)
	fmt.Println(saga.StepStatusPending)
	fmt.Println(saga.StepStatusCompleted)
	// Output:
	// reserve-inventory
	// pending
	// completed
}
