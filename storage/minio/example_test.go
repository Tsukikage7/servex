package minio_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/storage/minio"
)

func ExampleDefaultConfig() {
	cfg := minio.DefaultConfig()
	fmt.Println("region:", cfg.Region)
	fmt.Println("use ssl:", cfg.UseSSL)
	// Output:
	// region: us-east-1
	// use ssl: false
}

func ExampleConfig_Validate() {
	cfg := &minio.Config{Endpoint: "localhost:9000", Bucket: "test"}
	fmt.Println("valid:", cfg.Validate() == nil)

	// Missing endpoint.
	bad := &minio.Config{Bucket: "test"}
	fmt.Println("no endpoint:", bad.Validate())
	// Output:
	// valid: true
	// no endpoint: minio: endpoint is empty
}

func ExampleConfig_ApplyDefaults() {
	cfg := &minio.Config{
		Endpoint: "minio:9000",
		Bucket:   "data",
	}
	cfg.ApplyDefaults()

	fmt.Println("region:", cfg.Region)
	// Output:
	// region: us-east-1
}
