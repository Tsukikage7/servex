package secure_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/Tsukikage7/servex/v2/middleware/secure"
)

func ExampleHTTPMiddleware() {
	// Create secure middleware with default config.
	handler := secure.HTTPMiddleware(nil)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	fmt.Println("X-Frame-Options:", rec.Header().Get("X-Frame-Options"))
	fmt.Println("X-Content-Type-Options:", rec.Header().Get("X-Content-Type-Options"))
	fmt.Println("Referrer-Policy:", rec.Header().Get("Referrer-Policy"))
	// Output:
	// X-Frame-Options: DENY
	// X-Content-Type-Options: nosniff
	// Referrer-Policy: strict-origin-when-cross-origin
}

func ExampleDefaultConfig() {
	cfg := secure.DefaultConfig()
	fmt.Println("frame options:", cfg.XFrameOptions)
	fmt.Println("nosniff:", cfg.ContentTypeNosniff)
	fmt.Println("hsts max-age:", cfg.HSTSMaxAge)
	// Output:
	// frame options: DENY
	// nosniff: true
	// hsts max-age: 31536000
}
