package discovery_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/discovery"
)

func ExampleConfig_Validate() {
	cfg := &discovery.Config{
		Type: discovery.TypeConsul,
		Addr: "localhost:8500",
	}
	err := cfg.Validate()
	fmt.Println(err)
	// Output:
	// <nil>
}

func ExampleConfig_SetDefaults() {
	cfg := &discovery.Config{Type: discovery.TypeConsul}
	cfg.SetDefaults()

	svc := cfg.GetServiceConfig(discovery.ProtocolHTTP)
	fmt.Println(svc.Version)
	fmt.Println(svc.Protocol)
	// Output:
	// 1.0.0
	// http
}
