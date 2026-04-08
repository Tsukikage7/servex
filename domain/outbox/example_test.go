package outbox_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/domain/outbox"
	"github.com/Tsukikage7/servex/messaging/pubsub"
)

func ExampleNewOutboxMessage() {
	msg := &pubsub.Message{
		Topic:   "order.created",
		Key:     []byte("order-123"),
		Body:    []byte(`{"id":"order-123"}`),
		Headers: map[string]string{"source": "order-service"},
	}

	om := outbox.NewOutboxMessage(msg)
	fmt.Println(om.Topic)
	fmt.Println(string(om.Key))
	fmt.Println(string(om.Value))
	fmt.Println(om.Status)
	// Output:
	// order.created
	// order-123
	// {"id":"order-123"}
	// Pending
}

func ExampleMessageStatus_String() {
	fmt.Println(outbox.StatusPending)
	fmt.Println(outbox.StatusSent)
	fmt.Println(outbox.StatusFailed)
	// Output:
	// Pending
	// Sent
	// Failed
}
