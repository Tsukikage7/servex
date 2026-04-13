package httpclient

import (
	"net/http/httptest"
	"testing"

	"errors"

	servexerrors "github.com/Tsukikage7/servex/v2/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newResponse(status int, body string) *Response {
	rec := httptest.NewRecorder()
	rec.WriteHeader(status)
	rec.WriteString(body)
	return &Response{Response: rec.Result()}
}

func TestResponse_JSON(t *testing.T) {
	resp := newResponse(200, `{"id":1,"name":"test"}`)
	var target struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	require.NoError(t, resp.JSON(&target))
	assert.Equal(t, 1, target.ID)
	assert.Equal(t, "test", target.Name)
}

func TestResponse_JSON_InvalidBody(t *testing.T) {
	resp := newResponse(200, `not json`)
	var target map[string]any
	assert.Error(t, resp.JSON(&target))
}

func TestResponse_Text(t *testing.T) {
	resp := newResponse(200, "hello world")
	text, err := resp.Text()
	require.NoError(t, err)
	assert.Equal(t, "hello world", text)
}

func TestResponse_Bytes(t *testing.T) {
	resp := newResponse(200, "binary data")
	b, err := resp.Bytes()
	require.NoError(t, err)
	assert.Equal(t, []byte("binary data"), b)
}

func TestResponse_CheckStatus_2xx(t *testing.T) {
	for _, code := range []int{200, 201, 204} {
		resp := newResponse(code, "")
		assert.NoError(t, resp.CheckStatus())
	}
}

func TestResponse_CheckStatus_4xx(t *testing.T) {
	resp := newResponse(404, "")
	err := resp.CheckStatus()
	require.Error(t, err)

	e, ok := errors.AsType[*servexerrors.Error](err)
	require.True(t, ok)
	assert.Equal(t, 404, e.Code)
	assert.Equal(t, 404, e.HTTP)
}

func TestResponse_CheckStatus_5xx(t *testing.T) {
	resp := newResponse(500, "")
	err := resp.CheckStatus()
	require.Error(t, err)

	e, ok := errors.AsType[*servexerrors.Error](err)
	require.True(t, ok)
	assert.Equal(t, 500, e.Code)
	assert.Equal(t, 500, e.HTTP)
}

func TestResponse_CheckStatus_ErrorsAs(t *testing.T) {
	resp := newResponse(403, "")
	err := resp.CheckStatus()
	e, ok := errors.AsType[*servexerrors.Error](err)
	require.True(t, ok)
	assert.Equal(t, "http.403", e.Key)
	assert.Contains(t, e.Message, "403")
}
