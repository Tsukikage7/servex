package wechat_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/oauth2/wechat"
)

func ExampleNewProvider() {
	p := wechat.NewProvider(
		wechat.WithAppID("wx-app-id"),
		wechat.WithAppSecret("wx-app-secret"),
	)
	fmt.Println(p != nil)
	// Output:
	// true
}
