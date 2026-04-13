package logger_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

func ExampleNewDevConfig() {
	cfg := logger.NewDevConfig()
	fmt.Println(cfg.Level)
	fmt.Println(cfg.Format)
	// Output:
	// debug
	// console
}

func ExampleNewLogger() {
	log, err := logger.NewLogger(&logger.Config{
		Level:  logger.LevelInfo,
		Format: logger.FormatJSON,
	})
	fmt.Println(err)
	fmt.Println(log != nil)
	// Output:
	// <nil>
	// true
}
