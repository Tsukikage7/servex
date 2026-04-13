package app_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/app"
	"github.com/Tsukikage7/servex/v2/observability/logger"
)

func ExampleNew() {
	log := logger.MustNewLogger(&logger.Config{Level: "info", Format: "console", Output: "console"})
	application := app.New(app.WithLogger(log), app.WithName("my-service"))
	_ = application
	fmt.Println("app created")
	// Output: app created
}

func ExampleApplication_Name() {
	log := logger.MustNewLogger(&logger.Config{Level: "info", Format: "console", Output: "console"})
	application := app.New(
		app.WithLogger(log),
		app.WithName("order-service"),
		app.WithVersion("2.0.0"),
	)
	fmt.Println(application.Name())
	fmt.Println(application.Version())
	// Output:
	// order-service
	// 2.0.0
}
