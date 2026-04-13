package httpclient

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"errors"

	servexerrors "github.com/Tsukikage7/servex/v2/errors"
	"github.com/Tsukikage7/servex/v2/middleware/retry"
)

func TestNewSimple(t *testing.T) {
	c := NewSimple(WithBaseURL("http://example.com"))
	if c == nil {
		t.Fatal("client should not be nil")
	}
}

func TestNewSimple_Get(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"id": 1})
	}))
	defer srv.Close()

	c := NewSimple(WithBaseURL(srv.URL))
	resp, err := c.DoRequest(t.Context(), &Request{Method: http.MethodGet, Path: "/api"})
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := resp.JSON(&result); err != nil {
		t.Fatal(err)
	}
	if result["id"] != float64(1) {
		t.Errorf("id = %v", result["id"])
	}
}

func TestNewSimple_Post_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content-type = %s", r.Header.Get("Content-Type"))
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "test" {
			t.Errorf("name = %s", body["name"])
		}
		w.WriteHeader(201)
	}))
	defer srv.Close()

	c := NewSimple(WithBaseURL(srv.URL))
	resp, err := c.DoRequest(t.Context(), &Request{
		Method: http.MethodPost, Path: "/api",
		Body: map[string]string{"name": "test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 201 {
		t.Errorf("status = %d", resp.StatusCode)
	}
}

func TestNewSimple_MarshalError(t *testing.T) {
	c := NewSimple(WithBaseURL("http://example.com"))
	_, err := c.DoRequest(t.Context(), &Request{
		Method: http.MethodPost, Path: "/api",
		Body: make(chan int),
	})
	if !errors.Is(err, ErrMarshalBody) {
		t.Errorf("got %v, want ErrMarshalBody", err)
	}
}

func TestNewSimple_QueryParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") != "1" {
			t.Errorf("page = %s", r.URL.Query().Get("page"))
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := NewSimple(WithBaseURL(srv.URL))
	resp, err := c.DoRequest(t.Context(), &Request{
		Method: http.MethodGet, Path: "/api",
		Query: map[string]string{"page": "1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status = %d", resp.StatusCode)
	}
}

func TestNewSimple_PerRequestHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Request-ID") != "123" {
			t.Errorf("X-Request-ID = %s", r.Header.Get("X-Request-ID"))
		}
		if r.Header.Get("X-Service") != "order" {
			t.Errorf("X-Service = %s", r.Header.Get("X-Service"))
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := NewSimple(WithBaseURL(srv.URL), WithHeader("X-Service", "order"))
	resp, err := c.DoRequest(t.Context(), &Request{
		Method: http.MethodGet, Path: "/api",
		Headers: map[string]string{"X-Request-ID": "123"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status = %d", resp.StatusCode)
	}
}

func TestNewSimple_CheckStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()

	c := NewSimple(WithBaseURL(srv.URL))
	resp, err := c.DoRequest(t.Context(), &Request{Method: http.MethodGet, Path: "/missing"})
	if err != nil {
		t.Fatal(err)
	}
	err = resp.CheckStatus()
	e, ok := errors.AsType[*servexerrors.Error](err)
	if !ok {
		t.Fatalf("expected *errors.Error, got %T", err)
	}
	if e.Code != 404 {
		t.Errorf("code = %d", e.Code)
	}
}

func TestNewSimple_WithRetry(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(200)
		io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()

	c := NewSimple(
		WithBaseURL(srv.URL),
		WithRetry(&retry.Config{MaxAttempts: 3, Delay: time.Millisecond, Backoff: retry.FixedBackoff}),
	)
	resp, err := c.DoRequest(t.Context(), &Request{Method: http.MethodGet, Path: "/api"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status = %d", resp.StatusCode)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}
