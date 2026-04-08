package s3_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/storage/s3"
)

func ExampleDefaultConfig() {
	cfg := s3.DefaultConfig()
	fmt.Println("region:", cfg.Region)
	fmt.Println("use ssl:", cfg.UseSSL)
	fmt.Println("part size:", cfg.PartSize)
	// Output:
	// region: us-east-1
	// use ssl: true
	// part size: 5242880
}

func ExampleConfig_Validate() {
	cfg := &s3.Config{Endpoint: "http://s3.example.com", Bucket: "my-bucket"}
	fmt.Println("valid:", cfg.Validate() == nil)

	// Missing endpoint.
	bad := &s3.Config{Bucket: "my-bucket"}
	fmt.Println("no endpoint:", bad.Validate())
	// Output:
	// valid: true
	// no endpoint: s3: endpoint is empty
}

func ExampleConfig_ApplyDefaults() {
	cfg := &s3.Config{
		Endpoint: "http://s3.example.com",
		Bucket:   "my-bucket",
	}
	cfg.ApplyDefaults()

	fmt.Println("region:", cfg.Region)
	fmt.Println("max retries:", cfg.MaxRetries)
	// Output:
	// region: us-east-1
	// max retries: 3
}
