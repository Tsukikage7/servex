package ginserver_test

import (
	"context"
	"fmt"

	"github.com/Tsukikage7/servex/transport/ginserver"
)

type pingReq struct{}

type pingResp struct {
	Message string `json:"message"`
}

func ExampleHandle() {
	h := ginserver.Handle(func(_ context.Context, _ pingReq) (pingResp, error) {
		return pingResp{Message: "pong"}, nil
	})
	fmt.Println(h != nil)
	// Output:
	// true
}
