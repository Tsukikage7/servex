package sse_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/transport/sse"
)

func ExampleEvent_Bytes() {
	event := &sse.Event{
		ID:    "1",
		Event: "message",
		Data:  []byte("hello world"),
	}

	fmt.Print(string(event.Bytes()))
	// Output:
	// id: 1
	// event: message
	// data: hello world
	//
}

func ExampleDefaultConfig() {
	cfg := sse.DefaultConfig()
	fmt.Println("buffer size:", cfg.BufferSize)
	fmt.Println("retry interval:", cfg.RetryInterval)
	// Output:
	// buffer size: 256
	// retry interval: 3000
}

func ExampleNewServer() {
	srv := sse.NewServer(nil)
	fmt.Println("server created:", srv != nil)
	fmt.Println("client count:", srv.Count())
	// Output:
	// server created: true
	// client count: 0
}
