package redis_test

import (
	"fmt"

	jqredis "github.com/Tsukikage7/servex/v2/messaging/jobqueue/redis"
)

func ExampleWithPrefix() {
	opt := jqredis.WithPrefix("myapp")
	fmt.Println(opt != nil)
	// Output:
	// true
}
