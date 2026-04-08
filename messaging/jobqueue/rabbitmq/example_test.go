package rabbitmq_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/messaging/jobqueue/rabbitmq"
)

func ExampleWithPrefix() {
	opt := rabbitmq.WithPrefix("myapp")
	fmt.Println(opt != nil)
	// Output:
	// true
}

func ExampleWithDurable() {
	opt := rabbitmq.WithDurable(true)
	fmt.Println(opt != nil)
	// Output:
	// true
}
