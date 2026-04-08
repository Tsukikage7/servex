package websocket_test

import (
	"fmt"
	"time"

	"github.com/Tsukikage7/servex/transport/websocket"
)

func ExampleDefaultConfig() {
	cfg := websocket.DefaultConfig()
	fmt.Println("read buffer:", cfg.ReadBufferSize)
	fmt.Println("write buffer:", cfg.WriteBufferSize)
	fmt.Println("max message:", cfg.MaxMessageSize)
	fmt.Println("compression:", cfg.EnableCompression)
	// Output:
	// read buffer: 1024
	// write buffer: 1024
	// max message: 524288
	// compression: true
}

func ExampleNewHub() {
	hub := websocket.NewHub(func(client websocket.Client, msg *websocket.Message) {
		// Echo handler.
		_ = client.Send(msg)
	})

	fmt.Println("hub created:", hub != nil)
	fmt.Println("client count:", hub.Count())
	// Output:
	// hub created: true
	// client count: 0
}

func ExampleMessage() {
	msg := &websocket.Message{
		Type:      websocket.TextMessage,
		Data:      []byte("hello"),
		Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	fmt.Println("type:", msg.Type)
	fmt.Println("data:", string(msg.Data))
	// Output:
	// type: 1
	// data: hello
}
