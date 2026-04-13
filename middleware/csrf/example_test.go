package csrf_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/Tsukikage7/servex/v2/middleware/csrf"
)

func ExampleHTTPMiddleware() {
	// Create CSRF middleware with default config.
	handler := csrf.HTTPMiddleware(nil)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// On GET, a token is generated and stored in context.
			token := csrf.TokenFromContext(r.Context())
			if token != "" {
				fmt.Println("token generated: true")
			}
		}),
	)

	// GET request generates a token.
	req := httptest.NewRequest(http.MethodGet, "/form", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify cookie is set.
	cookies := rec.Result().Cookies()
	hasCookie := false
	for _, c := range cookies {
		if c.Name == "_csrf" {
			hasCookie = true
		}
	}
	fmt.Println("csrf cookie set:", hasCookie)
	// Output:
	// token generated: true
	// csrf cookie set: true
}

func ExampleDefaultConfig() {
	cfg := csrf.DefaultConfig()
	fmt.Println("cookie name:", cfg.CookieName)
	fmt.Println("header name:", cfg.HeaderName)
	// Output:
	// cookie name: _csrf
	// header name: X-CSRF-Token
}
