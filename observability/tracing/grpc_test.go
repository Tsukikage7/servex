package tracing

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestMetadataCarrier(t *testing.T) {
	md := metadata.MD{}
	carrier := metadataCarrier(md)

	// Test Set and Get
	carrier.Set("test-key", "test-value")
	assert.Equal(t, "test-value", carrier.Get("test-key"))

	// Test Get non-existent key
	assert.Empty(t, carrier.Get("non-existent"))

	// Test Keys
	keys := carrier.Keys()
	assert.Contains(t, keys, "test-key")
}

func TestUnaryServerInterceptor(t *testing.T) {
	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	defer tp.Shutdown(t.Context())

	interceptor := UnaryServerInterceptor("test-service")

	tests := []struct {
		name        string
		handlerErr  error
		expectError bool
	}{
		{
			name:        "success",
			handlerErr:  nil,
			expectError: false,
		},
		{
			name:        "error",
			handlerErr:  errors.New("handler error"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &grpc.UnaryServerInfo{
				FullMethod: "/test.Service/Method",
			}

			handler := func(ctx context.Context, req any) (any, error) {
				// 验证 context 中有 span
				span := SpanFromContext(ctx)
				assert.NotNil(t, span)
				return "response", tt.handlerErr
			}

			ctx := t.Context()
			resp, err := interceptor(ctx, "request", info, handler)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, "response", resp)
			}
		})
	}
}

func TestUnaryServerInterceptor_WithMetadata(t *testing.T) {
	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	defer tp.Shutdown(t.Context())

	interceptor := UnaryServerInterceptor("test-service")

	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/Method",
	}

	handler := func(ctx context.Context, req any) (any, error) {
		return "response", nil
	}

	// 带 metadata 的 context
	md := metadata.MD{
		"traceparent": []string{"00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"},
	}
	ctx := metadata.NewIncomingContext(t.Context(), md)

	resp, err := interceptor(ctx, "request", info, handler)

	assert.NoError(t, err)
	assert.Equal(t, "response", resp)
}

func TestStreamServerInterceptor(t *testing.T) {
	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	defer tp.Shutdown(t.Context())

	interceptor := StreamServerInterceptor("test-service")

	tests := []struct {
		name        string
		handlerErr  error
		expectError bool
	}{
		{
			name:        "success",
			handlerErr:  nil,
			expectError: false,
		},
		{
			name:        "error",
			handlerErr:  errors.New("handler error"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &grpc.StreamServerInfo{
				FullMethod:     "/test.Service/StreamMethod",
				IsClientStream: true,
				IsServerStream: true,
			}

			handler := func(srv any, stream grpc.ServerStream) error {
				// 验证 context 中有 span
				span := SpanFromContext(stream.Context())
				assert.NotNil(t, span)
				return tt.handlerErr
			}

			mockStream := &mockServerStream{ctx: t.Context()}
			err := interceptor(nil, mockStream, info, handler)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnaryClientInterceptor(t *testing.T) {
	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	defer tp.Shutdown(t.Context())

	interceptor := UnaryClientInterceptor("test-service")

	tests := []struct {
		name        string
		invokerErr  error
		expectError bool
	}{
		{
			name:        "success",
			invokerErr:  nil,
			expectError: false,
		},
		{
			name:        "error",
			invokerErr:  errors.New("invoker error"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
				// 验证 metadata 中有追踪信息
				md, ok := metadata.FromOutgoingContext(ctx)
				assert.True(t, ok)
				assert.NotNil(t, md)
				return tt.invokerErr
			}

			ctx := t.Context()
			var reply string
			err := interceptor(ctx, "/test.Service/Method", "request", &reply, nil, invoker)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnaryClientInterceptor_WithExistingMetadata(t *testing.T) {
	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	defer tp.Shutdown(t.Context())

	interceptor := UnaryClientInterceptor("test-service")

	invoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		md, ok := metadata.FromOutgoingContext(ctx)
		assert.True(t, ok)
		// 验证原有 metadata 保留
		assert.Equal(t, []string{"value"}, md.Get("existing-key"))
		return nil
	}

	// 带已有 metadata 的 context
	md := metadata.MD{"existing-key": []string{"value"}}
	ctx := metadata.NewOutgoingContext(t.Context(), md)

	var reply string
	err := interceptor(ctx, "/test.Service/Method", "request", &reply, nil, invoker)
	assert.NoError(t, err)
}

func TestStreamClientInterceptor(t *testing.T) {
	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	defer tp.Shutdown(t.Context())

	interceptor := StreamClientInterceptor("test-service")

	t.Run("success", func(t *testing.T) {
		desc := &grpc.StreamDesc{
			ClientStreams: true,
			ServerStreams: true,
		}

		streamer := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			return &mockClientStream{}, nil
		}

		stream, err := interceptor(t.Context(), desc, nil, "/test.Service/StreamMethod", streamer)

		assert.NoError(t, err)
		assert.NotNil(t, stream)
	})

	t.Run("error", func(t *testing.T) {
		desc := &grpc.StreamDesc{}

		streamer := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			return nil, errors.New("streamer error")
		}

		stream, err := interceptor(t.Context(), desc, nil, "/test.Service/StreamMethod", streamer)

		assert.Error(t, err)
		assert.Nil(t, stream)
	})
}

func TestClientStreamWrapper_RecvMsg(t *testing.T) {
	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	defer tp.Shutdown(t.Context())

	tracer := otel.Tracer("test")
	_, span := tracer.Start(t.Context(), "test")

	t.Run("EOF", func(t *testing.T) {
		wrapper := &clientStreamWrapper{
			ClientStream: &mockClientStream{recvErr: errors.New("EOF")},
			span:         span,
		}

		var msg string
		err := wrapper.RecvMsg(&msg)
		assert.Error(t, err)
	})

	t.Run("other error", func(t *testing.T) {
		_, newSpan := tracer.Start(t.Context(), "test2")
		wrapper := &clientStreamWrapper{
			ClientStream: &mockClientStream{recvErr: errors.New("some error")},
			span:         newSpan,
		}

		var msg string
		err := wrapper.RecvMsg(&msg)
		assert.Error(t, err)
	})
}

func TestInjectGRPCMetadata(t *testing.T) {
	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	defer tp.Shutdown(t.Context())

	// 创建带 span 的 context
	ctx, span := StartSpan(t.Context(), "test-service", "test-operation")
	defer span.End()

	// 注入 metadata
	ctx = InjectGRPCMetadata(ctx)

	// 验证 metadata 存在
	md, ok := metadata.FromOutgoingContext(ctx)
	assert.True(t, ok)
	assert.NotNil(t, md)
}

func TestInjectGRPCMetadata_WithExistingMetadata(t *testing.T) {
	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	defer tp.Shutdown(t.Context())

	// 带已有 metadata 的 context
	md := metadata.MD{"existing-key": []string{"value"}}
	ctx := metadata.NewOutgoingContext(t.Context(), md)

	// 注入 metadata
	ctx = InjectGRPCMetadata(ctx)

	// 验证原有 metadata 保留
	newMD, ok := metadata.FromOutgoingContext(ctx)
	assert.True(t, ok)
	assert.Equal(t, []string{"value"}, newMD.Get("existing-key"))
}

func TestExtractGRPCMetadata(t *testing.T) {
	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	defer tp.Shutdown(t.Context())

	// 带 metadata 的 context
	md := metadata.MD{
		"traceparent": []string{"00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"},
	}
	ctx := metadata.NewIncomingContext(t.Context(), md)

	// 提取 metadata
	ctx = ExtractGRPCMetadata(ctx)

	// 不应该 panic
	assert.NotNil(t, ctx)
}

func TestExtractGRPCMetadata_NoMetadata(t *testing.T) {
	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	defer tp.Shutdown(t.Context())

	ctx := t.Context()

	// 提取 metadata无 metadata
	newCtx := ExtractGRPCMetadata(ctx)

	// 应该返回原 context
	assert.Equal(t, ctx, newCtx)
}

func TestServerStreamWrapper_Context(t *testing.T) {
	ctx := context.WithValue(t.Context(), "key", "value")
	wrapper := &serverStreamWrapper{
		ServerStream: &mockServerStream{ctx: t.Context()},
		ctx:          ctx,
	}

	assert.Equal(t, ctx, wrapper.Context())
}

// Mock implementations

type mockServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (m *mockServerStream) Context() context.Context {
	return m.ctx
}

func (m *mockServerStream) SendMsg(msg any) error {
	return nil
}

func (m *mockServerStream) RecvMsg(msg any) error {
	return nil
}

func (m *mockServerStream) SendHeader(md metadata.MD) error {
	return nil
}

func (m *mockServerStream) SetHeader(md metadata.MD) error {
	return nil
}

type mockClientStream struct {
	grpc.ClientStream
	recvErr error
}

func (m *mockClientStream) Context() context.Context {
	return context.Background()
}

func (m *mockClientStream) SendMsg(msg any) error {
	return nil
}

func (m *mockClientStream) RecvMsg(msg any) error {
	return m.recvErr
}

func (m *mockClientStream) CloseSend() error {
	return nil
}

func (m *mockClientStream) Header() (metadata.MD, error) {
	return nil, nil
}

func (m *mockClientStream) Trailer() metadata.MD {
	return nil
}
