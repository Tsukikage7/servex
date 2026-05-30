package ratelimit

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestUnaryServerInterceptor(t *testing.T) {
	t.Run("允许请求", func(t *testing.T) {
		limiter := NewTokenBucket(10, 10)
		interceptor := UnaryServerInterceptor(limiter)

		handler := func(ctx context.Context, req any) (any, error) {
			return "success", nil
		}

		resp, err := interceptor(t.Context(), nil, &grpc.UnaryServerInfo{}, handler)
		if err != nil {
			t.Errorf("不期望错误: %v", err)
		}
		if resp != "success" {
			t.Error("期望返回 success")
		}
	})

	t.Run("拒绝请求", func(t *testing.T) {
		limiter := NewTokenBucket(1, 1)
		interceptor := UnaryServerInterceptor(limiter)

		handler := func(ctx context.Context, req any) (any, error) {
			return "success", nil
		}

		// 第一个请求通过
		_, _ = interceptor(t.Context(), nil, &grpc.UnaryServerInfo{}, handler)

		// 第二个请求被限流
		_, err := interceptor(t.Context(), nil, &grpc.UnaryServerInfo{}, handler)

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("期望 gRPC 状态错误")
		}
		if st.Code() != codes.ResourceExhausted {
			t.Errorf("期望 ResourceExhausted，得到 %v", st.Code())
		}
	})
}

func TestKeyedUnaryServerInterceptor(t *testing.T) {
	t.Run("不同方法独立限流", func(t *testing.T) {
		limiters := make(map[string]Limiter)
		limiters["/service/Method1"] = NewTokenBucket(1, 1)
		limiters["/service/Method2"] = NewTokenBucket(1, 1)

		interceptor := KeyedUnaryServerInterceptor(
			MethodKeyFunc(),
			func(key string) Limiter { return limiters[key] },
		)

		handler := func(ctx context.Context, req any) (any, error) {
			return "success", nil
		}

		ctx := t.Context()

		// Method1 第一个请求通过
		_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/service/Method1"}, handler)
		if err != nil {
			t.Errorf("Method1 第一个请求不应该被限流: %v", err)
		}

		// Method1 第二个请求被限流
		_, err = interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/service/Method1"}, handler)
		if err == nil {
			t.Error("Method1 第二个请求应该被限流")
		}

		// Method2 第一个请求通过独立限流
		_, err = interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/service/Method2"}, handler)
		if err != nil {
			t.Errorf("Method2 第一个请求不应该被限流: %v", err)
		}
	})
}

func TestMethodKeyFunc(t *testing.T) {
	keyFunc := MethodKeyFunc()

	info := &grpc.UnaryServerInfo{FullMethod: "/package.Service/Method"}
	key := keyFunc(t.Context(), info)

	if key != "/package.Service/Method" {
		t.Errorf("期望 /package.Service/Method，得到 %s", key)
	}
}

func TestMetadataKeyFunc(t *testing.T) {
	keyFunc := MetadataKeyFunc("user-id")

	t.Run("存在metadata", func(t *testing.T) {
		md := metadata.Pairs("user-id", "12345")
		ctx := metadata.NewIncomingContext(t.Context(), md)

		key := keyFunc(ctx, &grpc.UnaryServerInfo{})
		if key != "12345" {
			t.Errorf("期望 12345，得到 %s", key)
		}
	})

	t.Run("不存在metadata", func(t *testing.T) {
		key := keyFunc(t.Context(), &grpc.UnaryServerInfo{})
		if key != "" {
			t.Errorf("期望空字符串，得到 %s", key)
		}
	})
}

func TestCompositeGRPCKeyFunc(t *testing.T) {
	keyFunc := CompositeGRPCKeyFunc(MethodKeyFunc(), MetadataKeyFunc("user-id"))

	md := metadata.Pairs("user-id", "12345")
	ctx := metadata.NewIncomingContext(t.Context(), md)
	info := &grpc.UnaryServerInfo{FullMethod: "/service/Method"}

	key := keyFunc(ctx, info)
	expected := "/service/Method:12345"

	if key != expected {
		t.Errorf("期望 %s，得到 %s", expected, key)
	}
}
