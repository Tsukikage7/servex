package gateway_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/transport"
)

func ExampleServer_config() {
	// Show gateway configuration structure.
	cfg := transport.GatewayConfig{
		Name: "api-gateway",
		GRPC: transport.GRPCConfig{Addr: ":9090"},
		HTTP: transport.HTTPConfig{Addr: ":8080"},
	}

	fmt.Println("name:", cfg.Name)
	fmt.Println("grpc addr:", cfg.GRPC.Addr)
	fmt.Println("http addr:", cfg.HTTP.Addr)
	// Output:
	// name: api-gateway
	// grpc addr: :9090
	// http addr: :8080
}
