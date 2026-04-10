package httpclient_test

import (
	"fmt"
	"time"

	"github.com/Tsukikage7/servex/transport/httpclient"
)

func ExampleNewSimple() {
	// Create a simple HTTP client without service discovery.
	client := httpclient.NewSimple(
		httpclient.WithBaseURL("https://api.example.com"),
		httpclient.WithTimeout(10*time.Second),
		httpclient.WithHeader("Accept", "application/json"),
	)

	fmt.Println("client created:", client != nil)
	// Output:
	// client created: true
}

func ExampleNewFromConfig() {
	cfg := &httpclient.Config{
		BaseURL: "https://api.example.com",
		Timeout: 5 * time.Second,
	}

	client, err := httpclient.NewFromConfig(cfg)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("client created:", client != nil)
	// Output:
	// client created: true
}
