package neo4j_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/storage/neo4j"
)

func ExampleDefaultConfig() {
	cfg := neo4j.DefaultConfig()
	fmt.Println("database:", cfg.Database)
	fmt.Println("pool size:", cfg.MaxConnectionPoolSize)
	// Output:
	// database: neo4j
	// pool size: 100
}

func ExampleConfig_Validate() {
	cfg := &neo4j.Config{URI: "neo4j://localhost:7687", Database: "neo4j"}
	fmt.Println("valid:", cfg.Validate() == nil)

	// Missing URI.
	bad := &neo4j.Config{Database: "neo4j"}
	fmt.Println("no uri:", bad.Validate())
	// Output:
	// valid: true
	// no uri: neo4j: URI is empty
}

func ExampleRecord_Get() {
	rec := &neo4j.Record{
		Keys:   []string{"name", "age"},
		Values: []any{"Alice", int64(30)},
	}

	name, ok := rec.Get("name")
	fmt.Println("name:", name, "found:", ok)

	age := rec.GetByIndex(1)
	fmt.Println("age:", age)

	_, ok = rec.Get("missing")
	fmt.Println("missing:", ok)
	// Output:
	// name: Alice found: true
	// age: 30
	// missing: false
}
