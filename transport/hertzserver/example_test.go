package hertzserver_test

import (
	"context"
	"fmt"

	"github.com/Tsukikage7/servex/v2/transport/hertzserver"
)

type echoReq struct {
	Text string `json:"text"`
}

type echoResp struct {
	Text string `json:"text"`
}

func ExampleHandle() {
	h := hertzserver.Handle(func(_ context.Context, req echoReq) (echoResp, error) {
		return echoResp{Text: req.Text}, nil
	})
	fmt.Println(h != nil)
	// Output:
	// true
}
