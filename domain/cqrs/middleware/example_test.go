package middleware_test

import (
	"context"
	"fmt"

	"github.com/Tsukikage7/servex/domain/cqrs"
	"github.com/Tsukikage7/servex/domain/cqrs/middleware"
	"github.com/Tsukikage7/servex/observability/logger"
)

// echoHandler 示例命令处理器.
type echoHandler struct{}

func (h *echoHandler) Handle(_ context.Context, cmd string) (string, string, error) {
	return cmd, "ok:" + cmd, nil
}

func ExampleCommandLogging() {
	// CommandLogging wraps a handler with logging middleware.
	// In production, the logger would record command name and duration.
	log, _ := logger.NewLogger(&logger.Config{Level: logger.LevelError, Format: logger.FormatJSON})

	handler := cqrs.ChainCommand[string, string](
		&echoHandler{},
		middleware.CommandLogging[string, string](log, "CreateUser"),
	)

	_, result, err := handler.Handle(context.Background(), "alice")
	fmt.Println(result)
	fmt.Println(err)
	// Output:
	// ok:alice
	// <nil>
}
