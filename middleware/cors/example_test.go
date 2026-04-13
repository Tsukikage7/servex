package cors_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/Tsukikage7/servex/v2/middleware/cors"
)

func ExampleHTTPMiddleware() {
	// Create CORS middleware with custom origins.
	handler := cors.HTTPMiddleware(
		cors.WithAllowOrigins("https://example.com"),
		cors.WithAllowMethods(http.MethodGet, http.MethodPost),
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Preflight request.
	req := httptest.NewRequest(http.MethodOptions, "/api", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	fmt.Println("status:", rec.Code)
	fmt.Println("allow-origin:", rec.Header().Get("Access-Control-Allow-Origin"))
	// Output:
	// status: 204
	// allow-origin: https://example.com
}

func ExampleHTTPMiddleware_wildcard() {
	// Default: allow all origins.
	handler := cors.HTTPMiddleware()(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://any-site.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	fmt.Println("allow-origin:", rec.Header().Get("Access-Control-Allow-Origin"))
	// Output:
	// allow-origin: *
}
