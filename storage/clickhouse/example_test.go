package clickhouse_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/storage/clickhouse"
)

func ExampleDefaultConfig() {
	cfg := clickhouse.DefaultConfig()
	fmt.Println("addrs:", cfg.Addrs)
	fmt.Println("database:", cfg.Database)
	fmt.Println("compression:", cfg.Compression)
	// Output:
	// addrs: [localhost:9000]
	// database: default
	// compression: lz4
}

func ExampleConfig_Validate() {
	cfg := &clickhouse.Config{Addrs: []string{"localhost:9000"}, Database: "default"}
	fmt.Println("valid:", cfg.Validate() == nil)

	// Empty addrs.
	bad := &clickhouse.Config{Database: "default"}
	fmt.Println("no addrs:", bad.Validate())
	// Output:
	// valid: true
	// no addrs: clickhouse: addrs is empty
}

func ExampleConfig_ApplyDefaults() {
	cfg := &clickhouse.Config{
		Addrs:    []string{"ch:9000"},
		Database: "analytics",
	}
	cfg.ApplyDefaults()

	fmt.Println("max open:", cfg.MaxOpenConns)
	fmt.Println("compression:", cfg.Compression)
	// Output:
	// max open: 10
	// compression: lz4
}
