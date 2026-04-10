package errors

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// httpResponse HTTP 错误响应 JSON 结构.
type httpResponse struct {
	Code     int               `json:"code"`
	Key      string            `json:"key"`
	Message  string            `json:"message"`
	Metadata map[string]string `json:"metadata,omitzero"`
}

// ToHTTPStatus 从 error 中提取 HTTP 状态码，默认 500.
func ToHTTPStatus(err error) int {
	e, ok := FromError(err)
	if !ok || e.HTTP == 0 {
		return http.StatusInternalServerError
	}
	return e.HTTP
}

// WriteError 将 *Error 写入 HTTP 响应.
func WriteError(w http.ResponseWriter, err *Error) {
	status := err.HTTP
	if status == 0 {
		status = http.StatusInternalServerError
	}

	resp := httpResponse{
		Code:     err.Code,
		Key:      err.Key,
		Message:  err.Message,
		Metadata: err.Metadata,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

// WriteErrorFrom 将 error 写入 HTTP 响应.
// 非 *Error 类型不暴露内部错误详情，仅返回通用错误消息并记录原始错误.
func WriteErrorFrom(w http.ResponseWriter, err error) {
	e, ok := FromError(err)
	if !ok {
		slog.Error("内部服务器错误", "error", err)
		e = &Error{
			Code:    900500,
			Key:     "internal",
			Message: "内部服务器错误",
			HTTP:    http.StatusInternalServerError,
		}
	}
	WriteError(w, e)
}

// HTTPErrorHandler 返回 HTTP 错误处理中间件.
func HTTPErrorHandler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
}
