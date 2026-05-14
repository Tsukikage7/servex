package httpserver

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	// 注册编解码器
	_ "github.com/Tsukikage7/servex/v2/encoding/json"
	_ "github.com/Tsukikage7/servex/v2/encoding/xml"
)

// CodecTestSuite httpserver 编解码器测试套件.
type CodecTestSuite struct {
	suite.Suite
}

func TestCodecSuite(t *testing.T) {
	suite.Run(t, new(CodecTestSuite))
}

// 测试用响应结构.
type testResponse struct {
	Name string `json:"name" xml:"name"`
	Code int    `json:"code" xml:"code"`
}

// 测试用请求结构.
type testRequest struct {
	Name string `json:"name" xml:"name"`
	Age  int    `json:"age" xml:"age"`
}

// === EncodeCodecResponse 测试 ===

func (s *CodecTestSuite) TestEncodeCodecResponse_JSON() {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Accept", "application/json")
	ctx := context.WithValue(s.T().Context(), requestContextKey{}, r)

	w := httptest.NewRecorder()
	resp := testResponse{Name: "test", Code: 200}

	err := EncodeCodecResponse(ctx, w, resp)
	s.NoError(err)
	s.Contains(w.Header().Get("Content-Type"), "application/json")

	var got testResponse
	s.NoError(json.Unmarshal(w.Body.Bytes(), &got))
	s.Equal("test", got.Name)
	s.Equal(200, got.Code)
}

func (s *CodecTestSuite) TestEncodeCodecResponse_XML() {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Accept", "application/xml")
	ctx := context.WithValue(s.T().Context(), requestContextKey{}, r)

	w := httptest.NewRecorder()
	resp := testResponse{Name: "xml-test", Code: 201}

	err := EncodeCodecResponse(ctx, w, resp)
	s.NoError(err)
	s.Contains(w.Header().Get("Content-Type"), "application/xml")

	var got testResponse
	s.NoError(xml.Unmarshal(w.Body.Bytes(), &got))
	s.Equal("xml-test", got.Name)
}

func (s *CodecTestSuite) TestEncodeCodecResponse_DefaultJSON() {
	// 没有 Accept 头时回退到 JSON
	ctx := s.T().Context()
	w := httptest.NewRecorder()
	resp := testResponse{Name: "default", Code: 0}

	err := EncodeCodecResponse(ctx, w, resp)
	s.NoError(err)
	s.Contains(w.Header().Get("Content-Type"), "json")
}

// === DecodeCodecRequest 测试 ===

func (s *CodecTestSuite) TestDecodeCodecRequest_JSON() {
	body := `{"name":"alice","age":30}`
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	decoder := DecodeCodecRequest[testRequest]()
	result, err := decoder(s.T().Context(), r)
	s.NoError(err)

	req, ok := result.(testRequest)
	s.True(ok)
	s.Equal("alice", req.Name)
	s.Equal(30, req.Age)
}

func (s *CodecTestSuite) TestDecodeCodecRequest_XML() {
	body := `<testRequest><name>bob</name><age>25</age></testRequest>`
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/xml")

	decoder := DecodeCodecRequest[testRequest]()
	result, err := decoder(s.T().Context(), r)
	s.NoError(err)

	req, ok := result.(testRequest)
	s.True(ok)
	s.Equal("bob", req.Name)
	s.Equal(25, req.Age)
}

func (s *CodecTestSuite) TestDecodeCodecRequest_EmptyBody() {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	r.Header.Set("Content-Type", "application/json")

	decoder := DecodeCodecRequest[testRequest]()
	_, err := decoder(s.T().Context(), r)
	s.Error(err) // JSON 解码空内容应报错
}

// === WithRequest 测试 ===

func (s *CodecTestSuite) TestWithRequest() {
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.Header.Set("Accept", "application/xml")

	ctx := WithRequest()(s.T().Context(), r)
	got := requestFromContext(ctx)
	s.NotNil(got)
	s.Equal("application/xml", got.Header.Get("Accept"))
}

func (s *CodecTestSuite) TestRequestFromContext_NoRequest() {
	r := requestFromContext(s.T().Context())
	s.NotNil(r) // 应返回空请求而非 nil
}

// === EncodeCodecResponse 边界测试 ===

func (s *CodecTestSuite) TestEncodeCodecResponse_MarshalError() {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Accept", "application/json")
	ctx := context.WithValue(s.T().Context(), requestContextKey{}, r)

	w := httptest.NewRecorder()
	// channel 不能被 JSON 序列化
	resp := make(chan int)

	err := EncodeCodecResponse(ctx, w, resp)
	s.Error(err)
}

// === DecodeCodecRequest 边界测试 ===

func (s *CodecTestSuite) TestDecodeCodecRequest_InvalidJSON() {
	body := `{"name": invalid}`
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	decoder := DecodeCodecRequest[testRequest]()
	_, err := decoder(s.T().Context(), r)
	s.Error(err)
}

type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) { return 0, errors.New("broken reader") }

func (s *CodecTestSuite) TestDecodeCodecRequest_ReadError() {
	r := httptest.NewRequest(http.MethodPost, "/", brokenReader{})
	r.Header.Set("Content-Type", "application/json")

	decoder := DecodeCodecRequest[testRequest]()
	_, err := decoder(s.T().Context(), r)
	s.Error(err)
	s.Contains(err.Error(), "broken reader")
}

// === contentTypeForCodec 测试 ===

func (s *CodecTestSuite) TestContentTypeForCodec() {
	tests := []struct {
		name     string
		expected string
	}{
		{"json", "application/json; charset=utf-8"},
		{"xml", "application/xml; charset=utf-8"},
		{"proto", "application/x-protobuf"},
		{"unknown", "application/octet-stream"},
	}

	for _, tc := range tests {
		s.Equal(tc.expected, contentTypeForCodec(tc.name), "codec: %s", tc.name)
	}
}
