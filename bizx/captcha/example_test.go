package captcha_test

import (
	"context"
	"fmt"
	"time"

	"github.com/Tsukikage7/servex/bizx/captcha"
)

func ExampleNewManager() {
	store := captcha.NewMemoryStore()
	mgr := captcha.NewManager(store,
		captcha.WithLength(4),
		captcha.WithExpiration(5*time.Minute),
		captcha.WithCooldown(0), // 关闭冷却以便演示
	)
	ctx := context.Background()

	// 生成验证码.
	code, err := mgr.Generate(ctx, "user@example.com")
	fmt.Println("generate err:", err)
	fmt.Println("code length:", len(code.Code))

	// 验证错误的验证码.
	err = mgr.Verify(ctx, "user@example.com", "0000")
	fmt.Println("wrong code:", err)

	// 验证正确的验证码.
	err = mgr.Verify(ctx, "user@example.com", code.Code)
	fmt.Println("correct code:", err)
	// Output:
	// generate err: <nil>
	// code length: 4
	// wrong code: captcha: invalid code
	// correct code: <nil>
}
