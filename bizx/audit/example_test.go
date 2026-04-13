package audit_test

import (
	"context"
	"fmt"

	"github.com/Tsukikage7/servex/v2/bizx/audit"
)

func ExampleNewLogger() {
	store := audit.NewMemoryStore()
	logger := audit.NewLogger(store)
	ctx := context.Background()

	// 记录审计日志.
	err := logger.Log(ctx, &audit.Entry{
		Actor:      "user-1",
		Action:     "update",
		Resource:   "order",
		ResourceID: "ORD-001",
		Changes: map[string]audit.Change{
			"status": {From: "pending", To: "paid"},
		},
	})
	fmt.Println("log:", err)

	// 查询审计日志.
	entries, _ := logger.Query(ctx, &audit.Filter{Actor: "user-1"})
	fmt.Println("count:", len(entries))
	fmt.Println("action:", entries[0].Action)
	fmt.Println("resource:", entries[0].Resource)
	// Output:
	// log: <nil>
	// count: 1
	// action: update
	// resource: order
}
