package gzip

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readGzip 解压 gzip 数据.
func readGzip(t *testing.T, data []byte) string {
	t.Helper()
	r, err := gzip.NewReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer r.Close()
	b, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(b)
}

func TestNew_CompressesLargeResponse(t *testing.T) {
	body := strings.Repeat("Hello, gzip!", 100)
	handler := New()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(body)) //nolint:errcheck
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "gzip", w.Header().Get("Content-Encoding"))
	assert.Contains(t, w.Header().Get("Vary"), "Accept-Encoding")

	got := readGzip(t, w.Body.Bytes())
	assert.Equal(t, body, got)
}

func TestNew_SkipsSmallResponse(t *testing.T) {
	body := "hi"
	handler := New()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body)) //nolint:errcheck
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Empty(t, w.Header().Get("Content-Encoding"))
	assert.Equal(t, body, w.Body.String())
}

func TestNew_SkipsWithoutAcceptEncoding(t *testing.T) {
	body := strings.Repeat("x", 1000)
	handler := New()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body)) //nolint:errcheck
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// 不设置 Accept-Encoding
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Empty(t, w.Header().Get("Content-Encoding"))
	assert.Equal(t, body, w.Body.String())
}

func TestNew_SkipsExcludedPath(t *testing.T) {
	body := strings.Repeat("x", 1000)
	handler := New(WithExcludePaths("/health", "/metrics"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body)) //nolint:errcheck
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Empty(t, w.Header().Get("Content-Encoding"))
	assert.Equal(t, body, w.Body.String())
}

func TestNew_SkipsExcludedContentType(t *testing.T) {
	body := strings.Repeat("x", 1000)
	handler := New(WithExcludeContentTypes("image/png"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte(body)) //nolint:errcheck
	}))

	req := httptest.NewRequest(http.MethodGet, "/image.png", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Empty(t, w.Header().Get("Content-Encoding"))
}

func TestNew_CustomMinLength(t *testing.T) {
	body := strings.Repeat("a", 50)
	handler := New(WithMinLength(10))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(body)) //nolint:errcheck
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, "gzip", w.Header().Get("Content-Encoding"))
	got := readGzip(t, w.Body.Bytes())
	assert.Equal(t, body, got)
}

func TestNew_CustomLevel(t *testing.T) {
	body := strings.Repeat("compress me", 100)
	handler := New(WithLevel(gzip.BestSpeed))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(body)) //nolint:errcheck
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, "gzip", w.Header().Get("Content-Encoding"))
	got := readGzip(t, w.Body.Bytes())
	assert.Equal(t, body, got)
}

func TestHandler_Convenience(t *testing.T) {
	body := strings.Repeat("handler test", 100)
	handler := Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(body)) //nolint:errcheck
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, "gzip", w.Header().Get("Content-Encoding"))
	got := readGzip(t, w.Body.Bytes())
	assert.Equal(t, body, got)
}

func TestNew_WriteHeaderStatusCode(t *testing.T) {
	body := strings.Repeat("created", 100)
	handler := New()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(body)) //nolint:errcheck
	}))

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "gzip", w.Header().Get("Content-Encoding"))
}

func TestNew_EmptyBody(t *testing.T) {
	handler := New()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Header().Get("Content-Encoding"))
}

func TestNew_NonExcludedPath(t *testing.T) {
	body := strings.Repeat("data", 200)
	handler := New(WithExcludePaths("/health"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(body)) //nolint:errcheck
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, "gzip", w.Header().Get("Content-Encoding"))
}

func TestDefaultOptions(t *testing.T) {
	o := defaultOptions()
	assert.Equal(t, gzip.DefaultCompression, o.Level)
	assert.Equal(t, 256, o.MinLength)
	assert.Nil(t, o.ExcludePaths)
	assert.Nil(t, o.ExcludeContentTypes)
}

func TestNew_MultipleWrites(t *testing.T) {
	chunk := strings.Repeat("chunk", 60)
	handler := New()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		// 多次写入
		for i := 0; i < 5; i++ {
			w.Write([]byte(chunk)) //nolint:errcheck
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, "gzip", w.Header().Get("Content-Encoding"))
	got := readGzip(t, w.Body.Bytes())
	assert.Equal(t, strings.Repeat(chunk, 5), got)
}

func TestNew_Flush(t *testing.T) {
	body := strings.Repeat("flush", 200)
	handler := New()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(body)) //nolint:errcheck
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, "gzip", w.Header().Get("Content-Encoding"))
}
