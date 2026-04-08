package etcd_test

import "fmt"

func Example() {
	// etcd.New requires an *clientv3.Client which needs a running etcd server.
	// This example shows how you would configure the source:
	//
	//   client, _ := clientv3.New(clientv3.Config{Endpoints: []string{"localhost:2379"}})
	//   src := etcd.New(client, "/config/app", etcd.WithFormat("yaml"))
	//   kvs, _ := src.Load()
	//
	// WithFormat sets the config format (default "json").
	fmt.Println("etcd source supports json and yaml formats")
	// Output:
	// etcd source supports json and yaml formats
}
