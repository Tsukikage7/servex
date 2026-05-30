package signature

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-secret-key"

func TestSign_Verify(t *testing.T) {
	body := []byte(`{"key":"value"}`)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	sig := Sign(body, timestamp, testSecret)
	assert.NotEmpty(t, sig)

	// 验证正确签名
	assert.True(t, Verify(body, timestamp, sig, testSecret))

	// 验证错误签名
	assert.False(t, Verify(body, timestamp, "wrong-signature", testSecret))

	// 验证错误密钥
	assert.False(t, Verify(body, timestamp, sig, "wrong-secret"))

	// 验证错误 body
	assert.False(t, Verify([]byte("other body"), timestamp, sig, testSecret))

	// 验证错误 timestamp
	assert.False(t, Verify(body, "0", sig, testSecret))

	// 空 body
	emptySig := Sign(nil, timestamp, testSecret)
	assert.True(t, Verify(nil, timestamp, emptySig, testSecret))
}

func TestHTTPMiddleware_Valid(t *testing.T) {
	cfg := DefaultConfig(testSecret)
	body := []byte(`{"action":"test"}`)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	// 中间件验签包含 method+path，因此签名也需包含
	sig := SignWithMethodPath(body, http.MethodPost, "/api/test", timestamp, testSecret)

	handler := HTTPMiddleware(cfg)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/test", bytes.NewReader(body))
	req.Header.Set("X-Signature", sig)
	req.Header.Set("X-Timestamp", timestamp)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "ok", rec.Body.String())
}

func TestHTTPMiddleware_MissingSignature(t *testing.T) {
	cfg := DefaultConfig(testSecret)
	handler := HTTPMiddleware(cfg)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	// 缺少签名头
	req := httptest.NewRequest(http.MethodPost, "/api/test", bytes.NewReader([]byte("body")))
	req.Header.Set("X-Timestamp", strconv.FormatInt(time.Now().Unix(), 10))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "missing signature")

	// 缺少时间戳头
	req = httptest.NewRequest(http.MethodPost, "/api/test", bytes.NewReader([]byte("body")))
	req.Header.Set("X-Signature", "some-sig")

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "missing timestamp")
}

func TestHTTPMiddleware_ExpiredTimestamp(t *testing.T) {
	cfg := DefaultConfig(testSecret)
	cfg.MaxAge = 1 * time.Second

	handler := HTTPMiddleware(cfg)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	body := []byte("body")
	// 使用过期时间戳
	oldTimestamp := strconv.FormatInt(time.Now().Add(-10*time.Second).Unix(), 10)
	sig := Sign(body, oldTimestamp, testSecret)

	req := httptest.NewRequest(http.MethodPost, "/api/test", bytes.NewReader(body))
	req.Header.Set("X-Signature", sig)
	req.Header.Set("X-Timestamp", oldTimestamp)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "timestamp expired")
}

func TestHTTPMiddleware_InvalidSignature(t *testing.T) {
	cfg := DefaultConfig(testSecret)

	handler := HTTPMiddleware(cfg)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	body := []byte("body")
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	req := httptest.NewRequest(http.MethodPost, "/api/test", bytes.NewReader(body))
	req.Header.Set("X-Signature", "invalid-signature")
	req.Header.Set("X-Timestamp", timestamp)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid signature")
}

func TestSignRequest(t *testing.T) {
	body := []byte(`{"data":"hello"}`)

	req, err := http.NewRequest(http.MethodPost, "https://example.com/api", bytes.NewReader(body))
	require.NoError(t, err)

	err = SignRequest(req, testSecret)
	require.NoError(t, err)

	// 验证 headers 已设置
	assert.NotEmpty(t, req.Header.Get("X-Signature"))
	assert.NotEmpty(t, req.Header.Get("X-Timestamp"))

	// 使用中间件验证签名
	cfg := DefaultConfig(testSecret)

	var handlerCalled bool
	handler := HTTPMiddleware(cfg)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
			w.WriteHeader(http.StatusOK)
		}),
	)

	// 需要重新构造 request因为 body 已被读取，路径须与原始请求一致
	req2 := httptest.NewRequest(http.MethodPost, "/api", bytes.NewReader(body))
	req2.Header = req.Header

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req2)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, handlerCalled)
}

func TestSignRequest_NilBody(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://example.com/api", nil)
	require.NoError(t, err)

	err = SignRequest(req, testSecret)
	require.NoError(t, err)

	assert.NotEmpty(t, req.Header.Get("X-Signature"))
	assert.NotEmpty(t, req.Header.Get("X-Timestamp"))
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig("secret")
	assert.Equal(t, "secret", cfg.Secret)
	assert.Equal(t, "X-Signature", cfg.HeaderName)
	assert.Equal(t, "X-Timestamp", cfg.TimestampHeader)
	assert.Equal(t, 5*time.Minute, cfg.MaxAge)
	assert.Equal(t, "sha256", cfg.Algorithm)
}
