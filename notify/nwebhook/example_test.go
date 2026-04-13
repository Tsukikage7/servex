package nwebhook_test

import (
	"fmt"
	"time"

	"github.com/Tsukikage7/servex/v2/notify/nwebhook"
)

func ExampleWithTimeout() {
	opt := nwebhook.WithTimeout(10 * time.Second)
	fmt.Println(opt != nil)
	// Output:
	// true
}

func ExampleWithRetry() {
	opt := nwebhook.WithRetry(3)
	fmt.Println(opt != nil)
	// Output:
	// true
}
