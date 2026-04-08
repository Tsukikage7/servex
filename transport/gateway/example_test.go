package gateway_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/transport"
)

func ExampleServer_config() {
	// Show gateway configuration structure.
	cfg := transport.GatewayConfig{
		Name:     "api-gateway",
		GRPCAddr: ":9090",
		HTTPAddr: ":8080",
	}

	fmt.Println("name:", cfg.Name)
	fmt.Println("grpc addr:", cfg.GRPCAddr)
	fmt.Println("http addr:", cfg.HTTPAddr)
	// Output:
	// name: api-gateway
	// grpc addr: :9090
	// http addr: :8080
}
