package kafka_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/messaging/jobqueue/kafka"
)

func ExampleWithPrefix() {
	opt := kafka.WithPrefix("myapp")
	fmt.Println(opt != nil)
	// Output:
	// true
}
