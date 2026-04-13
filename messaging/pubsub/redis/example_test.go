package redis_test

import (
	"fmt"

	psredis "github.com/Tsukikage7/servex/v2/messaging/pubsub/redis"
)

func ExampleWithMaxLen() {
	opt := psredis.WithMaxLen(1000, true)
	fmt.Println(opt != nil)
	// Output:
	// true
}
