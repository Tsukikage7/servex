package logging

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tsukikage7/servex/v2/testx"
)

func TestHTTPMiddleware(t *testing.T) {
	log := testx.NopLogger()
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello"))
	})

	mw := HTTPMiddleware(WithLogger(log))
	handler := mw(inner)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if w.Body.String() != "hello" {
		t.Fatalf("body = %q, want %q", w.Body.String(), "hello")
	}
}

func TestHTTPMiddlewareSkipPaths(t *testing.T) {
	log := testx.NopLogger()
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := HTTPMiddleware(WithLogger(log), WithSkipPaths("/health", "/metrics"))
	handler := mw(inner)

	// Request to skipped path should still work.
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestHTTPMiddlewareWithoutLogger(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := HTTPMiddleware()(inner)
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestShouldSkip(t *testing.T) {
	tests := []struct {
		path      string
		skipPaths []string
		want      bool
	}{
		{"/health", []string{"/health", "/metrics"}, true},
		{"/metrics", []string{"/health", "/metrics"}, true},
		{"/api/data", []string{"/health", "/metrics"}, false},
		{"/health", nil, false},
	}

	for _, tt := range tests {
		got := shouldSkip(tt.path, tt.skipPaths)
		if got != tt.want {
			t.Errorf("shouldSkip(%q, %v) = %v, want %v", tt.path, tt.skipPaths, got, tt.want)
		}
	}
}

func TestStatusRecorder(t *testing.T) {
	w := httptest.NewRecorder()
	rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

	rec.WriteHeader(http.StatusNotFound)
	if rec.status != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.status)
	}

	n, err := rec.Write([]byte("body"))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != 4 || rec.bytesWritten != 4 {
		t.Fatalf("bytesWritten = %d, want 4", rec.bytesWritten)
	}
}
