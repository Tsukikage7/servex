package clientip_test

import (
	"context"
	"fmt"

	"github.com/Tsukikage7/servex/v2/httpx/clientip"
)

func ExampleWithIP() {
	ctx := clientip.WithIP(context.Background(), &clientip.IP{
		Address: "10.0.0.1",
		Port:    "8080",
	})

	ip, ok := clientip.FromContext(ctx)
	fmt.Println(ok)
	fmt.Println(ip.Address)
	fmt.Println(clientip.GetIP(ctx))
	// Output:
	// true
	// 10.0.0.1
	// 10.0.0.1
}
