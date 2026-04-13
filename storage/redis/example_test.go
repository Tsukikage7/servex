package redis_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/storage/redis"
)

func ExampleDefaultConfig() {
	cfg := redis.DefaultConfig()
	fmt.Println("addr:", cfg.Addr)
	fmt.Println("db:", cfg.DB)
	fmt.Println("pool size:", cfg.PoolSize)
	fmt.Println("max retries:", cfg.MaxRetries)
	// Output:
	// addr: localhost:6379
	// db: 0
	// pool size: 10
	// max retries: 3
}

func ExampleConfig_Validate() {
	cfg := &redis.Config{Addr: "localhost:6379"}
	fmt.Println("valid:", cfg.Validate() == nil)

	bad := &redis.Config{}
	fmt.Println("empty addr:", bad.Validate())
	// Output:
	// valid: true
	// empty addr: redis: addr is empty
}

func ExampleConfig_ApplyDefaults() {
	cfg := &redis.Config{Addr: "redis:6379"}
	cfg.ApplyDefaults()

	fmt.Println("pool size:", cfg.PoolSize)
	fmt.Println("min idle:", cfg.MinIdleConns)
	// Output:
	// pool size: 10
	// min idle: 2
}
