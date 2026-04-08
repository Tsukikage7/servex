package kafka_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/messaging/pubsub/kafka"
)

func ExampleWithPublisherLogger() {
	opt := kafka.WithPublisherLogger(nil)
	fmt.Println(opt != nil)
	// Output:
	// true
}
