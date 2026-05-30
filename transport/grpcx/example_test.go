package grpcx_test

import (
	"context"
	"fmt"

	"google.golang.org/grpc/metadata"

	"github.com/Tsukikage7/servex/v2/transport/grpcx"
)

func ExampleGetMetadataValue() {
	// Create a context with incoming metadata.
	md := metadata.Pairs("x-request-id", "abc-123")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	value := grpcx.GetMetadataValue(ctx, "x-request-id")
	fmt.Println("request id:", value)

	// Missing key returns empty string.
	value = grpcx.GetMetadataValue(ctx, "x-missing")
	fmt.Println("missing:", value)
	// Output:
	// request id: abc-123
	// missing:
}

func ExampleSetOutgoingMetadata() {
	ctx := grpcx.SetOutgoingMetadata(context.Background(),
		"x-trace-id", "trace-001",
		"x-request-id", "req-001",
	)

	md, ok := metadata.FromOutgoingContext(ctx)
	fmt.Println("has metadata:", ok)
	fmt.Println("trace:", md.Get("x-trace-id")[0])
	fmt.Println("request:", md.Get("x-request-id")[0])
	// Output:
	// has metadata: true
	// trace: trace-001
	// request: req-001
}

func ExampleAppendOutgoingMetadata() {
	ctx := grpcx.SetOutgoingMetadata(context.Background(), "key1", "val1")
	ctx = grpcx.AppendOutgoingMetadata(ctx, "key2", "val2")

	md, _ := metadata.FromOutgoingContext(ctx)
	fmt.Println("key1:", md.Get("key1")[0])
	fmt.Println("key2:", md.Get("key2")[0])
	// Output:
	// key1: val1
	// key2: val2
}
