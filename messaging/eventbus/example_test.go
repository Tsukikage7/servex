package eventbus_test

import (
	"context"
	"fmt"
	"time"

	"github.com/Tsukikage7/servex/messaging/eventbus"
)

// userRegistered 是一个示例事件.
type userRegistered struct {
	UserID string
}

func (e userRegistered) Topic() string { return "user.registered" }

func ExampleNew() {
	bus := eventbus.New()
	defer bus.Close()
	fmt.Println("bus created")
	// Output: bus created
}

func ExampleBus_Subscribe() {
	bus := eventbus.New()
	defer bus.Close()

	// 订阅事件.
	unsubscribe := bus.Subscribe("user.registered", func(ctx context.Context, event eventbus.Event) error {
		e := event.(userRegistered)
		fmt.Println("new user:", e.UserID)
		return nil
	})
	_ = unsubscribe // 可调用 unsubscribe() 取消订阅

	// 同步发布事件.
	_ = bus.Publish(context.Background(), userRegistered{UserID: "u-001"})
	// Output: new user: u-001
}

func ExampleBus_PublishAsync() {
	bus := eventbus.New(eventbus.WithAsyncWorkers(2))

	done := make(chan struct{})
	bus.Subscribe("user.registered", func(ctx context.Context, event eventbus.Event) error {
		e := event.(userRegistered)
		fmt.Println("async handler:", e.UserID)
		close(done)
		return nil
	})

	// 异步发布事件.
	bus.PublishAsync(context.Background(), userRegistered{UserID: "u-002"})

	select {
	case <-done:
	case <-time.After(time.Second):
		fmt.Println("timeout")
	}

	bus.Close()
	// Output: async handler: u-002
}
