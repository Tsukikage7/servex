package recovery

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Tsukikage7/servex/v2/endpoint"
	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// mockLogger 测试用 mock logger.
type mockLogger struct {
	errorCalled bool
	lastMessage string
	lastFields  []logger.Field
}

func newMockLogger() *mockLogger                        { return &mockLogger{} }
func (m *mockLogger) Debug(args ...any)                 {}
func (m *mockLogger) Debugf(format string, args ...any) {}
func (m *mockLogger) Info(args ...any)                  {}
func (m *mockLogger) Infof(format string, args ...any)  {}
func (m *mockLogger) Warn(args ...any)                  {}
func (m *mockLogger) Warnf(format string, args ...any)  {}
func (m *mockLogger) Error(args ...any) {
	m.errorCalled = true
	if len(args) > 0 {
		if msg, ok := args[0].(string); ok {
			m.lastMessage = msg
		}
	}
	for i := 1; i < len(args); i++ {
		if f, ok := args[i].(logger.Field); ok {
			m.lastFields = append(m.lastFields, f)
		}
	}
}
func (m *mockLogger) Errorf(format string, args ...any)         { m.errorCalled = true }
func (m *mockLogger) Fatal(args ...any)                         {}
func (m *mockLogger) Fatalf(format string, args ...any)         {}
func (m *mockLogger) Panic(args ...any)                         {}
func (m *mockLogger) Panicf(format string, args ...any)         {}
func (m *mockLogger) With(fields ...logger.Field) logger.Logger { return m }
func (m *mockLogger) Sync() error                               { return nil }
func (m *mockLogger) Close() error                              { return nil }

// ctxWithLogger 将 mock logger 注入到 context 中.
func ctxWithLogger(ctx context.Context, l logger.Logger) context.Context {
	return logger.NewContext(ctx, l)
}

// TestHTTPMiddleware_NoPanic 测试无 panic 情况.
func TestHTTPMiddleware_NoPanic(t *testing.T) {
	log := newMockLogger()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	wrapped := HTTPMiddleware(WithLogger(log))(handler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if log.errorCalled {
		t.Error("error should not be called when no panic")
	}
}

// TestHTTPMiddleware_WithPanic 测试 panic 恢复.
func TestHTTPMiddleware_WithPanic(t *testing.T) {
	log := newMockLogger()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	wrapped := HTTPMiddleware(WithLogger(log))(handler)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req = req.WithContext(ctxWithLogger(req.Context(), log))
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
	if !log.errorCalled {
		t.Error("error should be called when panic")
	}
	if log.lastMessage != "HTTP 异常已恢复" {
		t.Errorf("unexpected message: %s", log.lastMessage)
	}
}

// TestHTTPMiddleware_NilLogger 测试未设置 logger 时回退到空日志.
func TestHTTPMiddleware_NilLogger(t *testing.T) {
	HTTPMiddleware()
}

// TestHTTPMiddleware_CustomHandler 测试自定义处理函数.
func TestHTTPMiddleware_CustomHandler(t *testing.T) {
	log := newMockLogger()
	handlerCalled := false

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("custom panic")
	})

	customHandler := func(ctx any, p any, stack []byte) error {
		handlerCalled = true
		return nil
	}

	wrapped := HTTPMiddleware(
		WithLogger(log),
		WithHandler(customHandler),
	)(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if !handlerCalled {
		t.Error("custom handler should be called")
	}
}

// TestHTTPRecoverFunc 测试简化版恢复函数.
func TestHTTPRecoverFunc(t *testing.T) {
	log := newMockLogger()
	handler := func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}

	wrapped := HTTPRecoverFunc(log, handler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

// TestUnaryServerInterceptor_NoPanic 测试无 panic 情况.
func TestUnaryServerInterceptor_NoPanic(t *testing.T) {
	log := newMockLogger()
	interceptor := UnaryServerInterceptor(WithLogger(log))

	handler := func(ctx context.Context, req any) (any, error) {
		return "response", nil
	}

	resp, err := interceptor(
		t.Context(),
		"request",
		&grpc.UnaryServerInfo{FullMethod: "/test/method"},
		handler,
	)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if resp != "response" {
		t.Errorf("unexpected response: %v", resp)
	}
	if log.errorCalled {
		t.Error("error should not be called when no panic")
	}
}

// TestUnaryServerInterceptor_WithPanic 测试 panic 恢复.
func TestUnaryServerInterceptor_WithPanic(t *testing.T) {
	log := newMockLogger()
	interceptor := UnaryServerInterceptor(WithLogger(log))

	handler := func(ctx context.Context, req any) (any, error) {
		panic("grpc panic")
	}

	resp, err := interceptor(
		ctxWithLogger(t.Context(), log),
		"request",
		&grpc.UnaryServerInfo{FullMethod: "/test/method"},
		handler,
	)

	if resp != nil {
		t.Errorf("expected nil response, got %v", resp)
	}
	if err == nil {
		t.Error("expected error when panic")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Error("expected gRPC status error")
	}
	if st.Code() != codes.Internal {
		t.Errorf("expected Internal code, got %v", st.Code())
	}
	if !log.errorCalled {
		t.Error("error should be called when panic")
	}
}

// TestUnaryServerInterceptor_NilLogger 测试未设置 logger 时回退到空日志.
func TestUnaryServerInterceptor_NilLogger(t *testing.T) {
	UnaryServerInterceptor()
}

// TestStreamServerInterceptor_NoPanic 测试无 panic 情况.
func TestStreamServerInterceptor_NoPanic(t *testing.T) {
	log := newMockLogger()
	interceptor := StreamServerInterceptor(WithLogger(log))

	handler := func(srv any, stream grpc.ServerStream) error {
		return nil
	}

	err := interceptor(
		nil,
		&mockServerStream{ctx: t.Context()},
		&grpc.StreamServerInfo{FullMethod: "/test/stream"},
		handler,
	)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if log.errorCalled {
		t.Error("error should not be called when no panic")
	}
}

// TestStreamServerInterceptor_WithPanic 测试 panic 恢复.
func TestStreamServerInterceptor_WithPanic(t *testing.T) {
	log := newMockLogger()
	interceptor := StreamServerInterceptor(WithLogger(log))

	handler := func(srv any, stream grpc.ServerStream) error {
		panic("stream panic")
	}

	err := interceptor(
		nil,
		&mockServerStream{ctx: ctxWithLogger(t.Context(), log)},
		&grpc.StreamServerInfo{FullMethod: "/test/stream"},
		handler,
	)

	if err == nil {
		t.Error("expected error when panic")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Error("expected gRPC status error")
	}
	if st.Code() != codes.Internal {
		t.Errorf("expected Internal code, got %v", st.Code())
	}
	if !log.errorCalled {
		t.Error("error should be called when panic")
	}
}

// TestStreamServerInterceptor_NilLogger 测试未设置 logger 时回退到空日志.
func TestStreamServerInterceptor_NilLogger(t *testing.T) {
	StreamServerInterceptor()
}

// TestEndpointMiddleware_NoPanic 测试无 panic 情况.
func TestEndpointMiddleware_NoPanic(t *testing.T) {
	log := newMockLogger()
	middleware := EndpointMiddleware(WithLogger(log))

	endpoint := func(ctx context.Context, request any) (any, error) {
		return "response", nil
	}

	wrapped := middleware(endpoint)
	resp, err := wrapped(t.Context(), "request")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if resp != "response" {
		t.Errorf("unexpected response: %v", resp)
	}
	if log.errorCalled {
		t.Error("error should not be called when no panic")
	}
}

// TestEndpointMiddleware_WithPanic 测试 panic 恢复.
func TestEndpointMiddleware_WithPanic(t *testing.T) {
	log := newMockLogger()
	middleware := EndpointMiddleware(WithLogger(log))

	endpoint := func(ctx context.Context, request any) (any, error) {
		panic("endpoint panic")
	}

	wrapped := middleware(endpoint)
	resp, err := wrapped(ctxWithLogger(t.Context(), log), "request")

	if resp != nil {
		t.Errorf("expected nil response, got %v", resp)
	}
	if err == nil {
		t.Error("expected error when panic")
	}

	panicErr, ok := errors.AsType[*PanicError](err)
	if !ok {
		t.Error("expected PanicError")
	}
	if panicErr.Value != "endpoint panic" {
		t.Errorf("unexpected panic value: %v", panicErr.Value)
	}
	if !log.errorCalled {
		t.Error("error should be called when panic")
	}
}

// TestEndpointMiddleware_NilLogger 测试未设置 logger 时回退到空日志.
func TestEndpointMiddleware_NilLogger(t *testing.T) {
	EndpointMiddleware()
}

// TestEndpointMiddleware_CustomHandler 测试自定义处理函数.
func TestEndpointMiddleware_CustomHandler(t *testing.T) {
	log := newMockLogger()
	customErr := errors.New("custom error")

	middleware := EndpointMiddleware(
		WithLogger(log),
		WithHandler(func(ctx any, p any, stack []byte) error {
			return customErr
		}),
	)

	endpoint := func(ctx context.Context, request any) (any, error) {
		panic("test")
	}

	wrapped := middleware(endpoint)
	_, err := wrapped(t.Context(), "request")

	if !errors.Is(err, customErr) {
		t.Errorf("expected custom error, got %v", err)
	}
}

// TestPanicError 测试 PanicError.
func TestPanicError(t *testing.T) {
	t.Run("Error", func(t *testing.T) {
		err := &PanicError{Value: "test panic"}
		if err.Error() != "panic: test panic" {
			t.Errorf("unexpected error message: %s", err.Error())
		}
	})

	t.Run("Unwrap_Error", func(t *testing.T) {
		inner := errors.New("inner error")
		err := &PanicError{Value: inner}
		if !errors.Is(err, inner) {
			t.Error("expected to unwrap inner error")
		}
	})

	t.Run("Unwrap_NonError", func(t *testing.T) {
		err := &PanicError{Value: "string value"}
		if err.Unwrap() != nil {
			t.Error("expected nil for non-error value")
		}
	})
}

// TestOptions 测试配置选项.
func TestOptions(t *testing.T) {
	t.Run("WithStackSize", func(t *testing.T) {
		o := &Options{}
		WithStackSize(128 * 1024)(o)
		if o.StackSize != 128*1024 {
			t.Errorf("unexpected stack size: %d", o.StackSize)
		}
	})

	t.Run("WithStackAll", func(t *testing.T) {
		o := &Options{}
		WithStackAll(true)(o)
		if !o.StackAll {
			t.Error("expected StackAll to be true")
		}
	})

	t.Run("defaultOptions", func(t *testing.T) {
		o := defaultOptions()
		if o.StackSize != 64*1024 {
			t.Errorf("unexpected default stack size: %d", o.StackSize)
		}
		if o.StackAll {
			t.Error("expected StackAll to be false by default")
		}
	})
}

// TestStreamType 测试流类型识别.
func TestStreamType(t *testing.T) {
	tests := []struct {
		name     string
		info     *grpc.StreamServerInfo
		expected string
	}{
		{"bidi", &grpc.StreamServerInfo{IsClientStream: true, IsServerStream: true}, "bidi"},
		{"client", &grpc.StreamServerInfo{IsClientStream: true, IsServerStream: false}, "client"},
		{"server", &grpc.StreamServerInfo{IsClientStream: false, IsServerStream: true}, "server"},
		{"unary", &grpc.StreamServerInfo{IsClientStream: false, IsServerStream: false}, "unary"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := streamType(tt.info)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestCaptureStack 测试堆栈捕获.
func TestCaptureStack(t *testing.T) {
	stack := captureStack(4096, false)
	if len(stack) == 0 {
		t.Error("expected non-empty stack")
	}
	if len(stack) > 4096 {
		t.Error("stack should not exceed specified size")
	}
}

// TestMiddlewareChain 测试中间件链.
func TestMiddlewareChain(t *testing.T) {
	log := newMockLogger()
	callOrder := []string{}

	// 创建一个记录调用顺序的中间件
	orderMiddleware := func(name string) endpoint.Middleware {
		return func(next endpoint.Endpoint) endpoint.Endpoint {
			return func(ctx context.Context, request any) (any, error) {
				callOrder = append(callOrder, name+"-before")
				resp, err := next(ctx, request)
				callOrder = append(callOrder, name+"-after")
				return resp, err
			}
		}
	}

	// 链接中间件
	ep := func(ctx context.Context, request any) (any, error) {
		callOrder = append(callOrder, "endpoint")
		return "ok", nil
	}

	// recovery 在最外层
	wrapped := endpoint.Chain(
		EndpointMiddleware(WithLogger(log)),
		orderMiddleware("first"),
		orderMiddleware("second"),
	)(ep)

	_, _ = wrapped(t.Context(), nil)

	expected := []string{"first-before", "second-before", "endpoint", "second-after", "first-after"}
	if len(callOrder) != len(expected) {
		t.Errorf("unexpected call order length: got %v, expected %v", callOrder, expected)
	}
	for i, v := range expected {
		if callOrder[i] != v {
			t.Errorf("unexpected call order at %d: got %s, expected %s", i, callOrder[i], v)
		}
	}
}

// mockServerStream 测试用 mock ServerStream.
type mockServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (m *mockServerStream) Context() context.Context {
	return m.ctx
}
