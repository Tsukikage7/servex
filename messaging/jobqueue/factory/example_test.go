package factory_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/messaging/jobqueue/factory"
)

func ExampleStoreConfig() {
	cfg := factory.StoreConfig{
		Type: "memory",
	}
	fmt.Println(cfg.Type)
	// Output:
	// memory
}
