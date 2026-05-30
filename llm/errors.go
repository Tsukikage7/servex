package llm

import (
	"errors"
	"fmt"
)

// 哨兵错误.
var (
	// ErrRateLimited 请求被限流HTTP 429.
	ErrRateLimited = errors.New("llm: 请求被限流")
	// ErrContextLength 上下文长度超出模型限制.
	ErrContextLength = errors.New("llm: 上下文长度超出模型限制")
	// ErrInvalidAuth API 密钥无效或未授权.
	ErrInvalidAuth = errors.New("llm: API 密钥无效")
	// ErrProviderUnavailable Provider 服务不可用HTTP 5xx.
	ErrProviderUnavailable = errors.New("llm: 提供商服务不可用")
	// ErrContentFiltered 内容被安全策略过滤.
	ErrContentFiltered = errors.New("llm: 内容被安全策略过滤")
	// ErrStreamClosed 流已关闭，不能继续读取.
	ErrStreamClosed = errors.New("llm: 流已关闭")
)

// APIError Provider API 返回的错误.
type APIError struct {
	// StatusCode HTTP 状态码.
	StatusCode int
	// Code Provider 定义的错误码.
	Code string
	// Message 错误消息.
	Message string
	// Provider Provider 名称如 "openai", "anthropic".
	Provider string
	// RetryAfter HTTP 429 时建议重试的秒数0 表示未指定.
	RetryAfter int
}

// Error 实现 error 接口.
func (e *APIError) Error() string {
	return fmt.Sprintf("llm[%s]: status=%d code=%s msg=%s", e.Provider, e.StatusCode, e.Code, e.Message)
}

// IsRetryable 判断错误是否可重试.
// HTTP 429 和 5xx 错误可重试.
func IsRetryable(err error) bool {
	if apiErr, ok := errors.AsType[*APIError](err); ok {
		return apiErr.StatusCode == 429 || apiErr.StatusCode >= 500
	}
	return errors.Is(err, ErrRateLimited) || errors.Is(err, ErrProviderUnavailable)
}
