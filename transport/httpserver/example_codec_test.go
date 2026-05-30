package httpserver_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/Tsukikage7/servex/v2/endpoint"
	"github.com/Tsukikage7/servex/v2/transport/httpserver"

	_ "github.com/Tsukikage7/servex/v2/encoding/json"
	_ "github.com/Tsukikage7/servex/v2/encoding/xml"
)

type greetRequest struct {
	Name string `json:"name" xml:"name"`
}

type greetResponse struct {
	Message string `json:"message" xml:"message"`
}

func ExampleEncodeCodecResponse() {
	// 定义 endpoint
	greetEndpoint := endpoint.Endpoint(func(_ context.Context, req any) (any, error) {
		r := req.(greetRequest)
		return greetResponse{Message: "Hello, " + r.Name}, nil
	})

	// 创建 handler自动内容协商
	handler := httpserver.NewEndpointHandler(
		greetEndpoint,
		httpserver.DecodeCodecRequest[greetRequest](),
		httpserver.EncodeCodecResponse,
		httpserver.WithBefore(httpserver.WithRequest()),
	)

	// 模拟 JSON 请求
	body := `{"name":"Alice"}`
	r := httptest.NewRequest(http.MethodPost, "/greet", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	var resp greetResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	fmt.Println(resp.Message)
	// Output: Hello, Alice
}

func ExampleDecodeCodecRequest() {
	// DecodeCodecRequest 根据 Content-Type 自动选择解码器
	decoder := httpserver.DecodeCodecRequest[greetRequest]()

	// JSON 请求
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Bob"}`))
	r.Header.Set("Content-Type", "application/json")

	result, _ := decoder(context.Background(), r)
	req := result.(greetRequest)
	fmt.Println(req.Name)
	// Output: Bob
}
