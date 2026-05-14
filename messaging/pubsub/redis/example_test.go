package redis_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/messaging/pubsub/redis"
)

func ExampleWithMaxLen() {
	opt := redis.WithMaxLen(1000, true)
	fmt.Println(opt != nil)
	// Output:
	// true
}
