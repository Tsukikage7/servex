package rdbms_test

import (
	"fmt"
	"time"

	"github.com/Tsukikage7/servex/storage/rdbms"
)

func ExampleDefaultConfig() {
	cfg := rdbms.DefaultConfig()
	fmt.Println("type:", cfg.Type)
	fmt.Println("slow threshold:", cfg.SlowThreshold)
	fmt.Println("pool max open:", cfg.Pool.MaxOpen)
	fmt.Println("pool max idle:", cfg.Pool.MaxIdle)
	// Output:
	// type: gorm
	// slow threshold: 200ms
	// pool max open: 100
	// pool max idle: 10
}

func ExampleConfig_Validate() {
	cfg := &rdbms.Config{Driver: "mysql", DSN: "user:pass@tcp(localhost:3306)/db"}
	fmt.Println("valid:", cfg.Validate() == nil)

	// Missing driver.
	bad := &rdbms.Config{DSN: "something"}
	fmt.Println("no driver:", bad.Validate())
	// Output:
	// valid: true
	// no driver: database: 驱动类型为空
}

func ExampleDefaultPoolConfig() {
	pool := rdbms.DefaultPoolConfig()
	fmt.Println("max open:", pool.MaxOpen)
	fmt.Println("max lifetime:", pool.MaxLifetime)
	// Output:
	// max open: 100
	// max lifetime: 1h0m0s
}

func ExampleConfig_ApplyDefaults() {
	cfg := &rdbms.Config{
		Driver: "mysql",
		DSN:    "user:pass@tcp(localhost)/db",
	}
	cfg.ApplyDefaults()

	fmt.Println("type:", cfg.Type)
	fmt.Println("slow threshold:", cfg.SlowThreshold)
	fmt.Println("max idle time:", cfg.Pool.MaxIdleTime > time.Duration(0))
	// Output:
	// type: gorm
	// slow threshold: 200ms
	// max idle time: true
}
