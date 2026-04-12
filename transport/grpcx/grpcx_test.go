package grpcx

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
)

// --- metadata 测试 ---

func TestGetMetadataValue(t *testing.T) {
	t.Run("存在的 key 返回第一个值", func(t *testing.T) {
		md := metadata.Pairs("x-request-id", "abc123", "x-request-id", "def456")
		ctx := metadata.NewIncomingContext(t.Context(), md)

		val := GetMetadataValue(ctx, "x-request-id")
		assert.Equal(t, "abc123", val)
	})

	t.Run("不存在的 key 返回空字符串", func(t *testing.T) {
		md := metadata.Pairs("x-request-id", "abc123")
		ctx := metadata.NewIncomingContext(t.Context(), md)

		val := GetMetadataValue(ctx, "x-trace-id")
		assert.Empty(t, val)
	})

	t.Run("无 metadata 的 context 返回空字符串", func(t *testing.T) {
		val := GetMetadataValue(t.Context(), "x-request-id")
		assert.Empty(t, val)
	})
}

func TestGetMetadataValues(t *testing.T) {
	t.Run("存在的 key 返回所有值", func(t *testing.T) {
		md := metadata.Pairs("x-tag", "tag1", "x-tag", "tag2", "x-tag", "tag3")
		ctx := metadata.NewIncomingContext(t.Context(), md)

		values := GetMetadataValues(ctx, "x-tag")
		assert.Equal(t, []string{"tag1", "tag2", "tag3"}, values)
	})

	t.Run("不存在的 key 返回 nil", func(t *testing.T) {
		md := metadata.Pairs("x-tag", "tag1")
		ctx := metadata.NewIncomingContext(t.Context(), md)

		values := GetMetadataValues(ctx, "x-other")
		assert.Nil(t, values)
	})

	t.Run("无 metadata 的 context 返回 nil", func(t *testing.T) {
		values := GetMetadataValues(t.Context(), "x-tag")
		assert.Nil(t, values)
	})
}

func TestSetOutgoingMetadata(t *testing.T) {
	ctx := SetOutgoingMetadata(t.Context(), "x-key", "value1")
	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(t, ok)
	assert.Equal(t, []string{"value1"}, md.Get("x-key"))
}

func TestAppendOutgoingMetadata(t *testing.T) {
	t.Run("追加到空 context", func(t *testing.T) {
		ctx := AppendOutgoingMetadata(t.Context(), "x-key", "value1")
		md, ok := metadata.FromOutgoingContext(ctx)
		require.True(t, ok)
		assert.Equal(t, []string{"value1"}, md.Get("x-key"))
	})

	t.Run("追加到已有 metadata", func(t *testing.T) {
		ctx := AppendOutgoingMetadata(t.Context(), "x-key", "value1")
		ctx = AppendOutgoingMetadata(ctx, "x-key", "value2")
		md, ok := metadata.FromOutgoingContext(ctx)
		require.True(t, ok)
		assert.Equal(t, []string{"value1", "value2"}, md.Get("x-key"))
	})
}

func TestCopyIncomingToOutgoing(t *testing.T) {
	t.Run("复制全部 metadata", func(t *testing.T) {
		inMD := metadata.Pairs("x-a", "1", "x-b", "2")
		ctx := metadata.NewIncomingContext(t.Context(), inMD)

		ctx = CopyIncomingToOutgoing(ctx)
		outMD, ok := metadata.FromOutgoingContext(ctx)
		require.True(t, ok)
		assert.Equal(t, []string{"1"}, outMD.Get("x-a"))
		assert.Equal(t, []string{"2"}, outMD.Get("x-b"))
	})

	t.Run("复制指定 key", func(t *testing.T) {
		inMD := metadata.Pairs("x-a", "1", "x-b", "2", "x-c", "3")
		ctx := metadata.NewIncomingContext(t.Context(), inMD)

		ctx = CopyIncomingToOutgoing(ctx, "x-a", "x-c")
		outMD, ok := metadata.FromOutgoingContext(ctx)
		require.True(t, ok)
		assert.Equal(t, []string{"1"}, outMD.Get("x-a"))
		assert.Equal(t, []string{"3"}, outMD.Get("x-c"))
		assert.Empty(t, outMD.Get("x-b"))
	})

	t.Run("无入站 metadata 时不修改 context", func(t *testing.T) {
		ctx := CopyIncomingToOutgoing(t.Context())
		_, ok := metadata.FromOutgoingContext(ctx)
		assert.False(t, ok)
	})

	t.Run("指定不存在的 key 不修改 context", func(t *testing.T) {
		inMD := metadata.Pairs("x-a", "1")
		ctx := metadata.NewIncomingContext(t.Context(), inMD)

		ctx = CopyIncomingToOutgoing(ctx, "x-nonexistent")
		_, ok := metadata.FromOutgoingContext(ctx)
		assert.False(t, ok)
	})
}

// --- errors 测试 ---

func TestError(t *testing.T) {
	err := Error(codes.NotFound, "资源不存在")
	assert.Error(t, err)
	assert.Equal(t, codes.NotFound, Code(err))
	assert.Equal(t, "资源不存在", Message(err))
}

func TestErrorf(t *testing.T) {
	err := Errorf(codes.InvalidArgument, "参数 %s 无效: %d", "age", -1)
	assert.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, Code(err))
	assert.Contains(t, Message(err), "参数 age 无效: -1")
}

func TestCode(t *testing.T) {
	t.Run("nil 错误返回 OK", func(t *testing.T) {
		assert.Equal(t, codes.OK, Code(nil))
	})

	t.Run("gRPC 错误返回对应 code", func(t *testing.T) {
		err := Error(codes.PermissionDenied, "禁止访问")
		assert.Equal(t, codes.PermissionDenied, Code(err))
	})
}

func TestMessage(t *testing.T) {
	t.Run("nil 错误返回空字符串", func(t *testing.T) {
		assert.Empty(t, Message(nil))
	})

	t.Run("gRPC 错误返回 message", func(t *testing.T) {
		err := Error(codes.Internal, "内部错误")
		assert.Equal(t, "内部错误", Message(err))
	})
}

func TestIsCode(t *testing.T) {
	err := NotFound("未找到")
	assert.True(t, IsCode(err, codes.NotFound))
	assert.False(t, IsCode(err, codes.Internal))
	assert.True(t, IsCode(nil, codes.OK))
}

func TestConvenienceErrors(t *testing.T) {
	tests := []struct {
		name     string
		fn       func(string) error
		expected codes.Code
	}{
		{"NotFound", NotFound, codes.NotFound},
		{"InvalidArgument", InvalidArgument, codes.InvalidArgument},
		{"PermissionDenied", PermissionDenied, codes.PermissionDenied},
		{"Unauthenticated", Unauthenticated, codes.Unauthenticated},
		{"Internal", Internal, codes.Internal},
		{"Unavailable", Unavailable, codes.Unavailable},
		{"AlreadyExists", AlreadyExists, codes.AlreadyExists},
		{"DeadlineExceeded", DeadlineExceeded, codes.DeadlineExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn("测试消息")
			assert.Error(t, err)
			assert.Equal(t, tt.expected, Code(err))
			assert.Equal(t, "测试消息", Message(err))
		})
	}
}

// --- stream 测试 ---

// mockServerStream 模拟 gRPC ServerStream.
type mockServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (m *mockServerStream) Context() context.Context {
	return m.ctx
}

func TestWrapServerStream(t *testing.T) {
	originalCtx := context.WithValue(t.Context(), struct{ key string }{"original"}, "value")
	mock := &mockServerStream{ctx: originalCtx}

	newCtx := context.WithValue(t.Context(), struct{ key string }{"new"}, "new-value")
	wrapped := WrapServerStream(mock, newCtx)

	// 验证包装后的 stream 返回新的 context
	assert.Equal(t, newCtx, wrapped.Context())
	assert.Equal(t, "new-value", wrapped.Context().Value(struct{ key string }{"new"}))
}
