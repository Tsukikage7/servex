package trace_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/Tsukikage7/servex/v2/middleware/trace"
)

func ExampleHTTPMiddleware() {
	// 使用默认配置创建 trace 中间件.
	handler := trace.HTTPMiddleware(nil)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			traceID := trace.TraceIDFromContext(r.Context())
			fmt.Println("trace id set:", traceID != "")
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	fmt.Println("X-Trace-ID set:", rec.Header().Get("X-Trace-ID") != "")
	// Output:
	// trace id set: true
	// X-Trace-ID set: true
}

func ExampleDefaultConfig() {
	cfg := trace.DefaultConfig()
	fmt.Println("trace header:", cfg.TraceIDHeader)
	// Output:
	// trace header: X-Trace-ID
}

func ExampleTraceIDFromContext() {
	// 空 context 返回空字符串.
	fmt.Println("empty:", trace.TraceIDFromContext(context.Background()))
	// Output:
	// empty:
}
