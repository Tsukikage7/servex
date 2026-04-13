package gzip_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/Tsukikage7/servex/v2/middleware/gzip"
)

func ExampleNew() {
	// Create gzip middleware with default settings.
	handler := gzip.New()(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Write enough data to trigger compression (> 256 bytes default).
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(w, strings.Repeat("hello world ", 100))
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	fmt.Println("content-encoding:", rec.Header().Get("Content-Encoding"))
	// Output:
	// content-encoding: gzip
}

func ExampleNew_noGzip() {
	// Without Accept-Encoding: gzip, response is not compressed.
	handler := gzip.New()(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "small")
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	fmt.Println("content-encoding:", rec.Header().Get("Content-Encoding"))
	// Output:
	// content-encoding:
}
