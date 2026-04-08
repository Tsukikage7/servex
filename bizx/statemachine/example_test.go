package statemachine_test

import (
	"context"
	"fmt"

	"github.com/Tsukikage7/servex/bizx/statemachine"
)

func ExampleNew() {
	sm := statemachine.New("pending", []statemachine.Transition{
		{From: "pending", Event: "pay", To: "paid"},
		{From: "paid", Event: "ship", To: "shipped"},
		{From: "shipped", Event: "deliver", To: "delivered"},
		{From: "pending", Event: "cancel", To: "cancelled"},
	})

	fmt.Println("initial:", sm.Current())
	fmt.Println("can pay:", sm.Can("pay"))
	fmt.Println("can ship:", sm.Can("ship"))

	_ = sm.Fire(context.Background(), "pay", nil)
	fmt.Println("after pay:", sm.Current())

	fmt.Println("can ship now:", sm.Can("ship"))
	fmt.Println("can cancel:", sm.Can("cancel"))
	// Output:
	// initial: pending
	// can pay: true
	// can ship: false
	// after pay: paid
	// can ship now: true
	// can cancel: false
}

func ExampleMachine_AvailableEvents() {
	sm := statemachine.New("pending", []statemachine.Transition{
		{From: "pending", Event: "pay", To: "paid"},
		{From: "pending", Event: "cancel", To: "cancelled"},
		{From: "paid", Event: "ship", To: "shipped"},
	})

	events := sm.AvailableEvents()
	fmt.Println("available:", len(events))
	// Output:
	// available: 2
}
