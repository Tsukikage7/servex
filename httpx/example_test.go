package httpx_test

import (
	"context"
	"fmt"

	"github.com/Tsukikage7/servex/v2/httpx"
	"github.com/Tsukikage7/servex/v2/httpx/clientip"
)

func ExampleFromContext() {
	// Build a context with IP info.
	ctx := clientip.WithIP(context.Background(), &clientip.IP{Address: "192.168.1.1"})

	info := httpx.FromContext(ctx)
	fmt.Println(info.IP.Address)
	// Output:
	// 192.168.1.1
}
