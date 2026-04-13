package pubsub_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/messaging/pubsub"
)

func ExampleMessage() {
	msg := &pubsub.Message{
		Topic:   "order.created",
		Key:     []byte("order-123"),
		Body:    []byte(`{"id":"order-123","status":"created"}`),
		Headers: map[string]string{"source": "order-service"},
	}
	fmt.Println(msg.Topic)
	fmt.Println(string(msg.Key))
	fmt.Println(msg.Headers["source"])
	// Output:
	// order.created
	// order-123
	// order-service
}
