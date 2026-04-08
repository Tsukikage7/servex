package trace_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/Tsukikage7/servex/middleware/trace"
)

func ExampleHTTPMiddleware() {
	// Create trace middleware with default config.
	handler := trace.HTTPMiddleware(nil)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			traceID := trace.TraceIDFromContext(r.Context())
			reqID := trace.RequestIDFromContext(r.Context())
			fmt.Println("trace id set:", traceID != "")
			fmt.Println("request id set:", reqID != "")
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	fmt.Println("X-Trace-ID set:", rec.Header().Get("X-Trace-ID") != "")
	fmt.Println("X-Request-ID set:", rec.Header().Get("X-Request-ID") != "")
	// Output:
	// trace id set: true
	// request id set: true
	// X-Trace-ID set: true
	// X-Request-ID set: true
}

func ExampleDefaultConfig() {
	cfg := trace.DefaultConfig()
	fmt.Println("trace header:", cfg.TraceIDHeader)
	fmt.Println("request header:", cfg.RequestIDHeader)
	// Output:
	// trace header: X-Trace-ID
	// request header: X-Request-ID
}

func ExampleTraceIDFromContext() {
	// Empty context returns empty string.
	fmt.Println("empty:", trace.TraceIDFromContext(context.Background()))
	// Output:
	// empty:
}
