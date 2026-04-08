package grpcclient_test

import (
	"fmt"
	"time"

	"github.com/Tsukikage7/servex/transport/grpcclient"
)

func ExampleWithName() {
	// Show gRPC client option creation.
	opt := grpcclient.WithName("user-service")
	fmt.Println("option created:", opt != nil)
	// Output:
	// option created: true
}

func ExampleWithKeepalive() {
	opt := grpcclient.WithKeepalive(60*time.Second, 20*time.Second)
	fmt.Println("option created:", opt != nil)
	// Output:
	// option created: true
}

func ExampleWithRetry() {
	opt := grpcclient.WithRetry(3, 100*time.Millisecond)
	fmt.Println("option created:", opt != nil)
	// Output:
	// option created: true
}
