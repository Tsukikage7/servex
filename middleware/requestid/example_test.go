package requestid_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/Tsukikage7/servex/middleware/requestid"
)

func ExampleHTTPMiddleware() {
	// Create request ID middleware with a fixed generator.
	handler := requestid.HTTPMiddleware(
		requestid.WithGenerator(func() string { return "test-id-123" }),
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := requestid.FromContext(r.Context())
		fmt.Println("found:", ok)
		fmt.Println("id:", id)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	fmt.Println("header:", rec.Header().Get("X-Request-Id"))
	// Output:
	// found: true
	// id: test-id-123
	// header: test-id-123
}

func ExampleFromContext() {
	// Empty context returns false.
	_, ok := requestid.FromContext(context.Background())
	fmt.Println("found:", ok)
	// Output:
	// found: false
}
