package errors

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testErrTokenExpired = NewWithKind(100401, "auth.token.expired", "令牌已过期", KindUnauthenticated)
	testErrInternal     = NewWithKind(900500, "internal", "服务内部错误", KindInternal)
)

func TestToHTTPStatus(t *testing.T) {
	t.Run("from *Error with HTTP set", func(t *testing.T) {
		assert.Equal(t, 401, ToHTTPStatus(testErrTokenExpired))
	})

	t.Run("from *Error without HTTP set", func(t *testing.T) {
		err := New(999, "unknown", "未知错误")
		assert.Equal(t, 500, ToHTTPStatus(err))
	})

	t.Run("from standard error", func(t *testing.T) {
		assert.Equal(t, 500, ToHTTPStatus(fmt.Errorf("plain")))
	})

	t.Run("from nil", func(t *testing.T) {
		assert.Equal(t, 500, ToHTTPStatus(nil))
	})

	t.Run("from wrapped *Error", func(t *testing.T) {
		wrapped := testErrTokenExpired.WithCause(fmt.Errorf("bad"))
		assert.Equal(t, 401, ToHTTPStatus(wrapped))
	})
}

func TestWriteError_Nil(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteError(rec, nil)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, float64(900500), body["code"])
	assert.Equal(t, "内部服务器错误", body["message"])
}

func TestWriteError_InternalMasksDetail(t *testing.T) {
	rec := httptest.NewRecorder()
	err := NewWithKind(50001, "db.error", "password=secret", KindInternal)

	WriteError(rec, err)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, float64(50001), body["code"])
	assert.Equal(t, "内部服务器错误", body["message"])
}
