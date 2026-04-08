package cache_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/storage/cache"
)

func ExampleNewRedisConfig() {
	cfg := cache.NewRedisConfig("localhost:6379")
	fmt.Println("type:", cfg.Type)
	fmt.Println("addr:", cfg.Addr)
	fmt.Println("pool size:", cfg.PoolSize)
	// Output:
	// type: redis
	// addr: localhost:6379
	// pool size: 10
}

func ExampleNewMemoryConfig() {
	cfg := cache.NewMemoryConfig()
	fmt.Println("type:", cfg.Type)
	fmt.Println("max size:", cfg.MaxSize)
	// Output:
	// type: memory
	// max size: 10000
}

func ExampleConfig_Validate() {
	// Valid Redis config.
	cfg := cache.NewRedisConfig("localhost:6379")
	fmt.Println("valid:", cfg.Validate() == nil)

	// Invalid type.
	bad := &cache.Config{Type: "invalid"}
	fmt.Println("invalid type:", bad.Validate() != nil)
	// Output:
	// valid: true
	// invalid type: true
}
