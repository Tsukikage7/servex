package errors

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func TestToGRPCStatus(t *testing.T) {
	errTokenExpired := New(100401, "auth.token.expired", "令牌已过期").
		WithHTTP(http.StatusUnauthorized).WithGRPC(codes.Unauthenticated)

	t.Run("from *Error", func(t *testing.T) {
		st := ToGRPCStatus(errTokenExpired)
		assert.Equal(t, codes.Unauthenticated, st.Code())
		assert.Contains(t, st.Message(), "令牌已过期")
	})

	t.Run("from *Error without GRPC set", func(t *testing.T) {
		err := New(999, "unknown", "未知错误")
		st := ToGRPCStatus(err)
		assert.Equal(t, codes.Internal, st.Code())
	})

	t.Run("from standard error", func(t *testing.T) {
		st := ToGRPCStatus(fmt.Errorf("plain error"))
		assert.Equal(t, codes.Internal, st.Code())
		assert.Equal(t, "plain error", st.Message())
	})

	t.Run("from nil", func(t *testing.T) {
		st := ToGRPCStatus(nil)
		assert.Equal(t, codes.OK, st.Code())
	})
}

func TestToGRPCStatus_ErrorInfo(t *testing.T) {
	err := New(100401, "auth.token.expired", "令牌已过期").
		WithHTTP(http.StatusUnauthorized).
		WithGRPC(codes.Unauthenticated).
		WithMeta("user_id", "123")

	st := ToGRPCStatus(err)
	require.Equal(t, codes.Unauthenticated, st.Code())

	// 验证 ErrorInfo detail 存在
	var found bool
	for _, d := range st.Details() {
		if ei, ok := d.(*errdetails.ErrorInfo); ok {
			found = true
			assert.Equal(t, "auth.token.expired", ei.Reason)
			assert.Equal(t, "servex", ei.Domain)
			assert.Equal(t, "123", ei.Metadata["user_id"])
		}
	}
	assert.True(t, found, "应在 status details 中找到 ErrorInfo")
}

func TestToGRPCStatus_BadRequest(t *testing.T) {
	err := New(100400, "validation.failed", "参数校验失败").
		WithGRPC(codes.InvalidArgument).
		WithMeta("field", "email").
		WithMeta("field_violation", "邮箱格式无效")

	st := ToGRPCStatus(err)

	// 验证 BadRequest detail 存在
	var found bool
	for _, d := range st.Details() {
		if br, ok := d.(*errdetails.BadRequest); ok {
			found = true
			require.Len(t, br.FieldViolations, 1)
			assert.Equal(t, "email", br.FieldViolations[0].Field)
			assert.Equal(t, "邮箱格式无效", br.FieldViolations[0].Description)
		}
	}
	assert.True(t, found, "应在 status details 中找到 BadRequest")
}

func TestToGRPCStatus_RetryInfo(t *testing.T) {
	err := New(100429, "rate.limited", "请求过于频繁").
		WithGRPC(codes.ResourceExhausted).
		WithMeta("retry_delay", "5s")

	st := ToGRPCStatus(err)

	// 验证 RetryInfo detail 存在
	var found bool
	for _, d := range st.Details() {
		if ri, ok := d.(*errdetails.RetryInfo); ok {
			found = true
			require.NotNil(t, ri.RetryDelay)
			assert.Equal(t, int64(5), ri.RetryDelay.Seconds)
		}
	}
	assert.True(t, found, "应在 status details 中找到 RetryInfo")
}

func TestToGRPCStatus_AllDetails(t *testing.T) {
	// 同时包含 BadRequest 和 RetryInfo
	err := New(100400, "validation.failed", "参数校验失败").
		WithGRPC(codes.InvalidArgument).
		WithMeta("field", "email").
		WithMeta("field_violation", "邮箱格式无效").
		WithMeta("retry_delay", "3s")

	st := ToGRPCStatus(err)

	var hasErrorInfo, hasBadRequest, hasRetryInfo bool
	for _, d := range st.Details() {
		switch d.(type) {
		case *errdetails.ErrorInfo:
			hasErrorInfo = true
		case *errdetails.BadRequest:
			hasBadRequest = true
		case *errdetails.RetryInfo:
			hasRetryInfo = true
		}
	}
	assert.True(t, hasErrorInfo, "应包含 ErrorInfo")
	assert.True(t, hasBadRequest, "应包含 BadRequest")
	assert.True(t, hasRetryInfo, "应包含 RetryInfo")
}

func TestFromGRPCStatus(t *testing.T) {
	t.Run("round trip", func(t *testing.T) {
		original := New(100401, "auth.token.expired", "令牌已过期").
			WithHTTP(401).WithGRPC(codes.Unauthenticated)
		st := ToGRPCStatus(original)

		restored := FromGRPCStatus(st)
		require.NotNil(t, restored)
		assert.Equal(t, 100401, restored.Code)
		assert.Equal(t, "auth.token.expired", restored.Key)
		assert.Equal(t, "令牌已过期", restored.Message)
	})

	t.Run("from plain grpc status", func(t *testing.T) {
		st := grpcstatus.New(codes.NotFound, "not found")
		restored := FromGRPCStatus(st)
		require.NotNil(t, restored)
		assert.Equal(t, int(codes.NotFound), restored.Code)
		assert.Equal(t, "not found", restored.Message)
	})
}

func TestFromGRPCStatus_WithErrorInfo(t *testing.T) {
	original := New(100401, "auth.token.expired", "令牌已过期").
		WithGRPC(codes.Unauthenticated).
		WithMeta("user_id", "456")

	st := ToGRPCStatus(original)

	restored := FromGRPCStatus(st)
	require.NotNil(t, restored)
	assert.Equal(t, 100401, restored.Code)
	assert.Equal(t, "auth.token.expired", restored.Key)
	assert.Equal(t, "456", restored.Metadata["user_id"])
}

func TestFromGRPCStatus_WithBadRequest(t *testing.T) {
	original := New(100400, "validation.failed", "参数校验失败").
		WithGRPC(codes.InvalidArgument).
		WithMeta("field", "username").
		WithMeta("field_violation", "用户名不能为空")

	st := ToGRPCStatus(original)

	restored := FromGRPCStatus(st)
	require.NotNil(t, restored)
	assert.Equal(t, "username", restored.Metadata["field"])
	assert.Equal(t, "用户名不能为空", restored.Metadata["field_violation"])
}

func TestFromGRPCStatus_WithRetryInfo(t *testing.T) {
	original := New(100429, "rate.limited", "请求过于频繁").
		WithGRPC(codes.ResourceExhausted).
		WithMeta("retry_delay", "10s")

	st := ToGRPCStatus(original)

	restored := FromGRPCStatus(st)
	require.NotNil(t, restored)
	assert.Equal(t, "10s", restored.Metadata["retry_delay"])
}

func TestFromGRPCStatus_Nil(t *testing.T) {
	assert.Nil(t, FromGRPCStatus(nil))
}

func TestFromGRPCStatus_OK(t *testing.T) {
	st := grpcstatus.New(codes.OK, "")
	assert.Nil(t, FromGRPCStatus(st))
}

func TestUnaryServerInterceptor(t *testing.T) {
	errTokenExpired := New(100401, "auth.token.expired", "令牌已过期").
		WithHTTP(http.StatusUnauthorized).WithGRPC(codes.Unauthenticated)
	interceptor := UnaryServerInterceptor()

	t.Run("handler returns *Error", func(t *testing.T) {
		handler := func(ctx context.Context, req any) (any, error) {
			return nil, errTokenExpired
		}
		_, err := interceptor(t.Context(), nil, &grpc.UnaryServerInfo{}, handler)
		require.Error(t, err)

		st, ok := grpcstatus.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Unauthenticated, st.Code())
	})

	t.Run("handler returns nil", func(t *testing.T) {
		handler := func(ctx context.Context, req any) (any, error) {
			return "ok", nil
		}
		resp, err := interceptor(t.Context(), nil, &grpc.UnaryServerInfo{}, handler)
		assert.NoError(t, err)
		assert.Equal(t, "ok", resp)
	})

	t.Run("handler returns standard error", func(t *testing.T) {
		handler := func(ctx context.Context, req any) (any, error) {
			return nil, fmt.Errorf("plain error")
		}
		_, err := interceptor(t.Context(), nil, &grpc.UnaryServerInfo{}, handler)
		require.Error(t, err)

		st, ok := grpcstatus.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Internal, st.Code())
	})
}
