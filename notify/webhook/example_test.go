package webhook_test

import (
	"fmt"
	"time"

	"github.com/Tsukikage7/servex/notify/webhook"
)

func ExampleEvent() {
	event := webhook.Event{
		ID:        "evt-1",
		Type:      "order.created",
		Payload:   []byte(`{"id":"123"}`),
		Timestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	fmt.Println(event.Type)
	// Output:
	// order.created
}

func ExampleNewHMACSigner() {
	signer := webhook.NewHMACSigner()
	sig := signer.Sign([]byte("payload"), "secret")
	fmt.Println(signer.Verify([]byte("payload"), "secret", sig))
	// Output:
	// true
}
