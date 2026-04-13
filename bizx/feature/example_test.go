package feature_test

import (
	"context"
	"fmt"

	"github.com/Tsukikage7/servex/v2/bizx/feature"
)

func ExampleNewManager() {
	store := feature.NewMemoryStore()
	mgr := feature.NewManager(store)
	ctx := context.Background()

	// 创建全局开启的特性开关.
	_ = mgr.SetFlag(ctx, &feature.Flag{
		Name:    "dark-mode",
		Enabled: true,
	})

	// 创建白名单模式的特性开关.
	_ = mgr.SetFlag(ctx, &feature.Flag{
		Name:    "new-dashboard",
		Enabled: true,
		Users:   []string{"user-1", "user-2"},
	})

	// 评估特性开关.
	fmt.Println("dark-mode:", mgr.IsEnabled(ctx, "dark-mode"))
	fmt.Println("new-dashboard (user-1):", mgr.IsEnabled(ctx, "new-dashboard", feature.WithUser("user-1")))
	fmt.Println("new-dashboard (user-99):", mgr.IsEnabled(ctx, "new-dashboard", feature.WithUser("user-99")))
	fmt.Println("non-existent:", mgr.IsEnabled(ctx, "non-existent"))
	// Output:
	// dark-mode: true
	// new-dashboard (user-1): true
	// new-dashboard (user-99): false
	// non-existent: false
}
