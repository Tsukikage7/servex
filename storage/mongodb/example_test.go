package mongodb_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/storage/mongodb"
)

func ExampleDefaultConfig() {
	cfg := mongodb.DefaultConfig()
	fmt.Println("max pool:", cfg.MaxPoolSize)
	fmt.Println("min pool:", cfg.MinPoolSize)
	// Output:
	// max pool: 100
	// min pool: 5
}

func ExampleConfig_Validate() {
	cfg := &mongodb.Config{URI: "mongodb://localhost:27017", Database: "mydb"}
	fmt.Println("valid:", cfg.Validate() == nil)

	// Missing URI.
	bad := &mongodb.Config{Database: "mydb"}
	fmt.Println("no uri:", bad.Validate())
	// Output:
	// valid: true
	// no uri: mongodb: URI is empty
}

func ExampleNewObjectID() {
	id := mongodb.NewObjectID()
	fmt.Println("id generated:", id.Hex() != "")
	// Output:
	// id generated: true
}
