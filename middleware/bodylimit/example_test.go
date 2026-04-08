package bodylimit_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/Tsukikage7/servex/middleware/bodylimit"
)

func ExampleHTTPMiddleware() {
	// Create body limit middleware: max 1 MB.
	handler := bodylimit.HTTPMiddleware(1 << 20)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	// Request without body passes.
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	fmt.Println("status:", rec.Code)
	// Output:
	// status: 200
}

func ExampleParseLimit() {
	bytes, err := bodylimit.ParseLimit("10MB")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("bytes:", bytes)
	// Output:
	// bytes: 10485760
}
