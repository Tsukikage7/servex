package endpoint_test

import (
	"context"
	"fmt"

	"github.com/Tsukikage7/servex/endpoint"
)

func ExampleChain() {
	// Create a simple endpoint.
	ep := func(_ context.Context, req any) (any, error) {
		return fmt.Sprintf("handled:%v", req), nil
	}

	// Create middleware that adds a prefix.
	addPrefix := func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req any) (any, error) {
			return next(ctx, fmt.Sprintf("prefixed(%v)", req))
		}
	}

	// Create middleware that adds a suffix.
	addSuffix := func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req any) (any, error) {
			resp, err := next(ctx, req)
			return fmt.Sprintf("%v:suffixed", resp), err
		}
	}

	chained := endpoint.Chain(addSuffix, addPrefix)(ep)
	resp, _ := chained(context.Background(), "hello")
	fmt.Println(resp)
	// Output:
	// handled:prefixed(hello):suffixed
}
