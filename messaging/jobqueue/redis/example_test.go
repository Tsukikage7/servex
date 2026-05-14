package redis_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/messaging/jobqueue/redis"
)

func ExampleWithPrefix() {
	opt := redis.WithPrefix("myapp")
	fmt.Println(opt != nil)
	// Output:
	// true
}
