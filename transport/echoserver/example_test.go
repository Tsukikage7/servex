package echoserver_test

import (
	"context"
	"fmt"

	"github.com/Tsukikage7/servex/v2/transport/echoserver"
)

type helloReq struct {
	Name string `json:"name"`
}

type helloResp struct {
	Greeting string `json:"greeting"`
}

func ExampleHandle() {
	h := echoserver.Handle(func(_ context.Context, req helloReq) (helloResp, error) {
		return helloResp{Greeting: "Hello, " + req.Name}, nil
	})
	fmt.Println(h != nil)
	// Output:
	// true
}
