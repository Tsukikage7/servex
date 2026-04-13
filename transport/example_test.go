package transport_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/transport"
)

func ExampleBuildMethodSkipper() {
	// Build a method skipper with exact and prefix patterns.
	skipper := transport.BuildMethodSkipper([]string{
		"/api.Auth/Login",
		"/api.Health/*",
	})

	fmt.Println("exact match:", skipper("/api.Auth/Login"))
	fmt.Println("prefix match:", skipper("/api.Health/Check"))
	fmt.Println("no match:", skipper("/api.User/GetProfile"))
	// Output:
	// exact match: true
	// prefix match: true
	// no match: false
}

func ExampleHTTPConfig() {
	cfg := transport.HTTPConfig{
		Name: "api",
		Addr: ":8080",
	}
	fmt.Println("name:", cfg.Name)
	fmt.Println("addr:", cfg.Addr)
	// Output:
	// name: api
	// addr: :8080
}
