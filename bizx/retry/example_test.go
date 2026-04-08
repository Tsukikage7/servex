package retry_test

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Tsukikage7/servex/bizx/retry"
)

func ExampleNewScheduler() {
	store := retry.NewMemoryStore()
	scheduler := retry.NewScheduler(store)
	ctx := context.Background()

	// 注册处理器.
	scheduler.Register("send_email", func(ctx context.Context, payload json.RawMessage) error {
		fmt.Println("processing:", string(payload))
		return nil
	})

	// 提交任务.
	id, err := scheduler.Submit(ctx, "send_email", map[string]string{
		"to":      "user@example.com",
		"subject": "Hello",
	}, retry.WithMaxRetries(3))

	fmt.Println("submit error:", err)
	fmt.Println("task id not empty:", id != "")
	// Output:
	// submit error: <nil>
	// task id not empty: true
}
