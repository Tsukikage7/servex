package factory_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/messaging/pubsub/factory"
)

func ExampleConfig() {
	cfg := factory.Config{
		Type: "redis",
	}
	fmt.Println(cfg.Type)
	// Output:
	// redis
}
