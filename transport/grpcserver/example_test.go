package grpcserver_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/observability/logger"
	"github.com/Tsukikage7/servex/transport/grpcserver"
)

func ExampleNew() {
	log := logger.MustNewLogger(&logger.Config{Level: "info", Format: "console", Output: "console"})

	srv := grpcserver.New(
		grpcserver.WithLogger(log),
		grpcserver.WithName("order-grpc"),
		grpcserver.WithAddr(":9090"),
		grpcserver.WithRecovery(),
		grpcserver.WithReflection(true),
	)

	fmt.Println(srv.Name())
	fmt.Println(srv.Addr())
	// Output:
	// order-grpc
	// :9090
}
