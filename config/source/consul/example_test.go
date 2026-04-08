package consul_test

import "fmt"

func Example() {
	// consul.New requires an *api.Client which needs a running Consul server.
	// This example shows how you would configure the source:
	//
	//   client, _ := api.NewClient(api.DefaultConfig())
	//   src := consul.New(client, "config/app", consul.WithFormat("yaml"), consul.WithDatacenter("dc1"))
	//   kvs, _ := src.Load()
	//
	// WithFormat sets the config format (default "json").
	fmt.Println("consul source supports json and yaml formats")
	// Output:
	// consul source supports json and yaml formats
}
