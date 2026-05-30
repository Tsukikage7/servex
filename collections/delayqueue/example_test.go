package delayqueue_test

import (
	"context"
	"fmt"
	"time"

	"github.com/Tsukikage7/servex/v2/collections/delayqueue"
)

// task 实现 Delayable 接口.
type task struct {
	name    string
	readyAt time.Time
}

func (t task) Delay() time.Duration {
	return time.Until(t.readyAt)
}

func ExampleNew() {
	dq := delayqueue.New[task]()
	ctx := context.Background()

	// 添加已到期的元素.
	_ = dq.Enqueue(ctx, task{name: "immediate", readyAt: time.Now().Add(-time.Second)})

	fmt.Println("len:", dq.Len())

	// 出队已到期的元素.
	item, err := dq.Dequeue(ctx)
	fmt.Println("dequeued:", item.name)
	fmt.Println("error:", err)
	// Output:
	// len: 1
	// dequeued: immediate
	// error: <nil>
}
