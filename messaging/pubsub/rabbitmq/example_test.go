package rabbitmq_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/messaging/pubsub/rabbitmq"
)

func ExampleWithExchange() {
	opt := rabbitmq.WithExchange("my-exchange", "topic")
	fmt.Println(opt != nil)
	// Output:
	// true
}
