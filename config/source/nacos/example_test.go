package nacos_test

import "fmt"

func Example() {
	// nacos.New requires a config_client.IConfigClient which needs a running Nacos server.
	// This example shows how you would configure the source:
	//
	//   src := nacos.New(client, "app.json",
	//       nacos.WithFormat("json"),
	//       nacos.WithGroup("DEFAULT_GROUP"),
	//       nacos.WithNamespace("public"),
	//   )
	//   kvs, _ := src.Load()
	//
	// WithFormat sets the config format (default "json").
	fmt.Println("nacos source supports format, group, and namespace options")
	// Output:
	// nacos source supports format, group, and namespace options
}
