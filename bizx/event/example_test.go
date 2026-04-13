package event_test

import (
	"context"
	"fmt"

	"github.com/Tsukikage7/servex/v2/bizx/event"
)

func ExampleNew() {
	bus := event.New()
	defer bus.Close()

	// 订阅事件.
	bus.Subscribe("user.created", func(_ context.Context, evt *event.Event) error {
		fmt.Println("event:", evt.Name)
		fmt.Println("payload:", evt.Payload)
		return nil
	})

	// 发布事件.
	_ = bus.Publish(context.Background(), "user.created", "alice")
	// Output:
	// event: user.created
	// payload: alice
}

func ExampleBus_Subscribe_wildcard() {
	bus := event.New()
	defer bus.Close()

	var received []string
	bus.Subscribe("order.*", func(_ context.Context, evt *event.Event) error {
		received = append(received, evt.Name)
		return nil
	})

	_ = bus.Publish(context.Background(), "order.created", nil)
	_ = bus.Publish(context.Background(), "order.paid", nil)
	_ = bus.Publish(context.Background(), "user.created", nil) // 不匹配

	fmt.Println("received:", len(received))
	fmt.Println(received[0])
	fmt.Println(received[1])
	// Output:
	// received: 2
	// order.created
	// order.paid
}
