package apollo_test

import "fmt"

func Example() {
	// apollo.New requires an agollo.Client which needs a running Apollo server.
	// This example shows how you would configure the source:
	//
	//   src := apollo.New("app-id", "default", "application",
	//       apollo.WithFormat("json"),
	//       apollo.WithCluster("default"),
	//   )
	//   kvs, _ := src.Load()
	//
	// WithFormat sets the config format (default "json").
	fmt.Println("apollo source supports configurable formats and clusters")
	// Output:
	// apollo source supports configurable formats and clusters
}
