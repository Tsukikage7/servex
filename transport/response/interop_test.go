package response_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Tsukikage7/servex/v2/errors"
	errorsgrpcx "github.com/Tsukikage7/servex/v2/errors/grpcx"
	"github.com/Tsukikage7/servex/v2/transport/response"
)

// 互操作测试：验证 servex/errors.Error 可以正确地通过 response 包处理.

func TestExtractCode_WithErrorsPackage(t *testing.T) {
	err := errors.NewWithKind(40001, "user.not_found", "用户不存在", errors.KindNotFound).
		WithMeta("user_id", "u-123")

	code := response.ExtractCode(err)
	assert.Equal(t, 40001, code.Num)
	assert.Equal(t, "user.not_found", code.Key)
	assert.Equal(t, "用户不存在", code.Message)
	assert.Equal(t, http.StatusNotFound, code.HTTPStatus())
	assert.Equal(t, codes.NotFound, code.GRPCCode())
}

func TestExtractCode_WithErrorsPackageMissingTransportMapping(t *testing.T) {
	err := errors.New(response.CodeNotFound.Num, response.CodeNotFound.Key, response.CodeNotFound.Message)

	code := response.ExtractCode(err)
	assert.Equal(t, response.CodeNotFound.Num, code.Num)
	assert.Equal(t, response.CodeNotFound.Key, code.Key)
	assert.Equal(t, response.CodeNotFound.Message, code.Message)
	assert.Equal(t, http.StatusNotFound, code.HTTPStatus())
	assert.Equal(t, codes.NotFound, code.GRPCCode())
}

func TestWriteError_WithErrorsPackageMissingTransportMapping(t *testing.T) {
	err := errors.New(response.CodeNotFound.Num, response.CodeNotFound.Key, response.CodeNotFound.Message)
	rec := httptest.NewRecorder()

	require.NoError(t, response.WriteError(rec, err, "en"))

	assert.Equal(t, http.StatusNotFound, rec.Code)
	var body response.Response[any]
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Equal(t, response.CodeNotFound.Num, body.Code)
	assert.Equal(t, "Resource not found", body.Message)
}

func TestGRPCStatus_WithErrorsPackageMissingTransportMapping(t *testing.T) {
	err := errors.New(response.CodeNotFound.Num, response.CodeNotFound.Key, response.CodeNotFound.Message)

	st := response.GRPCStatus(err)

	assert.Equal(t, codes.NotFound, st.Code())
	code := response.FromGRPCStatus(st)
	assert.Equal(t, response.CodeNotFound.Num, code.Num)
	assert.Equal(t, http.StatusNotFound, code.HTTPStatus())
	assert.Equal(t, codes.NotFound, code.GRPCCode())
}

func TestExtractMetadata_WithErrorsPackage(t *testing.T) {
	err := errors.New(40001, "user.not_found", "用户不存在").
		WithMeta("user_id", "u-123").
		WithMeta("request_id", "req-456")

	metadata := response.ExtractMetadata(err)
	require.NotNil(t, metadata)
	assert.Equal(t, "u-123", metadata["user_id"])
	assert.Equal(t, "req-456", metadata["request_id"])
}

func TestExtractMetadata_NoMetadata(t *testing.T) {
	err := errors.New(40001, "user.not_found", "用户不存在")
	metadata := response.ExtractMetadata(err)
	assert.Nil(t, metadata)
}

func TestExtractMetadata_NilError(t *testing.T) {
	assert.Nil(t, response.ExtractMetadata(nil))
}

func TestExtractMetadata_NonServexError(t *testing.T) {
	// 普通 error 不应有 metadata
	err := errors.New(40001, "", "")
	_ = err
	assert.Nil(t, response.ExtractMetadata(http.ErrServerClosed))
}

func TestExtractMessage_WithErrorsPackage(t *testing.T) {
	err := errors.NewWithKind(40001, "user.not_found", "用户不存在", errors.KindNotFound)

	msg := response.ExtractMessage(err)
	assert.Equal(t, "用户不存在", msg)
}

func TestExtractMessage_InternalError_Masked(t *testing.T) {
	// 内部错误 (>= 50000) 应该被掩码
	err := errors.NewWithKind(50001, "db.connection_failed", "数据库密码错误: xxx", errors.KindInternal)

	msg := response.ExtractMessage(err)
	// 应返回通用消息，不暴露敏感信息
	assert.NotEqual(t, "数据库密码错误: xxx", msg)
}

func TestGRPCStatus_WithErrorsPackage_AttachesDetails(t *testing.T) {
	err := errors.NewWithKind(40001, "user.not_found", "用户不存在", errors.KindNotFound).
		WithMeta("user_id", "u-123").
		WithMeta("field", "email").
		WithMeta("field_violation", "格式无效").
		WithMeta("retry_delay", "5s")

	st := response.GRPCStatus(err)
	assert.Equal(t, codes.NotFound, st.Code())

	// 验证 ErrorInfo detail 存在
	var foundErrorInfo bool
	var foundBadRequest bool
	var foundRetryInfo bool
	for _, d := range st.Details() {
		switch v := d.(type) {
		case *errdetails.ErrorInfo:
			foundErrorInfo = true
			assert.Equal(t, "user.not_found", v.Reason)
			assert.Equal(t, "servex", v.Domain)
			assert.Equal(t, "u-123", v.Metadata["user_id"])
		case *errdetails.BadRequest:
			foundBadRequest = true
			require.Len(t, v.FieldViolations, 1)
			assert.Equal(t, "email", v.FieldViolations[0].Field)
		case *errdetails.RetryInfo:
			foundRetryInfo = true
			assert.NotNil(t, v.RetryDelay)
		}
	}
	assert.True(t, foundErrorInfo, "应附加 ErrorInfo detail")
	assert.True(t, foundBadRequest, "应附加 BadRequest detail（因 metadata 含 field）")
	assert.True(t, foundRetryInfo, "应附加 RetryInfo detail（因 metadata 含 retry_delay）")
}

func TestFromGRPCStatus_ErrorsPackageFormat(t *testing.T) {
	// errors/grpcx.ToGRPCStatus 生成的 status，response.FromGRPCStatus 应能识别
	original := errors.NewWithKind(40001, "user.not_found", "用户不存在", errors.KindNotFound)

	st := errorsgrpcx.ToGRPCStatus(original)
	code := response.FromGRPCStatus(st)

	assert.Equal(t, 40001, code.Num)
	assert.Equal(t, "user.not_found", code.Key)
	assert.Equal(t, "用户不存在", code.Message)
}

func TestUnaryServerInterceptor_WithErrorsPackage(t *testing.T) {
	// 验证 response.UnaryServerInterceptor 能处理 servex/errors.Error
	interceptor := response.UnaryServerInterceptor()

	handler := func(_ context.Context, _ any) (any, error) {
		return nil, errors.NewWithKind(40001, "user.not_found", "用户不存在", errors.KindNotFound)
	}

	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, handler)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())

	// 验证能从 status 恢复 Code
	code := response.FromGRPCStatus(st)
	assert.Equal(t, 40001, code.Num)
}

func TestLocalizedMessage_WithErrorsPackage(t *testing.T) {
	// servex/errors.Error 的 Key 应该能被 i18n 翻译
	err := errors.NewWithKind(40001, "error.not_found", "资源不存在", errors.KindNotFound)

	// 使用内置英文翻译Key 为 "error.not_found" 时翻译为 "Resource not found"
	msg := response.LocalizedMessage(err, "en")
	assert.Equal(t, "Resource not found", msg)
}

func TestLocalizedMessage_WithErrorsPackage_CustomKey(t *testing.T) {
	// 未注册的 Key，应回退到 Message
	err := errors.New(40001, "custom.key.not.in.bundle", "自定义消息")
	msg := response.LocalizedMessage(err, "en")
	// 翻译失败，应回退到 Message
	assert.Equal(t, "自定义消息", msg)
}

func TestLocalizedMessage_InternalError_WithErrorsPackage(t *testing.T) {
	// 内部错误不应暴露 Message
	err := errors.New(50001, "db.error", "password=secret123 connection failed")
	msg := response.LocalizedMessage(err, "zh-CN")
	assert.NotContains(t, msg, "secret123")
}
