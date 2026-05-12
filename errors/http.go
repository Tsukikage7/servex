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
	if !ok {
		return http.StatusInternalServerError
	}
	return e.Kind.HTTPStatus()
}

// WriteError 将 *Error 写入 HTTP 响应.
func WriteError(w http.ResponseWriter, err *Error) {
	if err == nil {
		err = internalHTTPError()
	}

	resp := httpResponse{
		Code:     err.Code,
		Key:      err.Key,
		Message:  httpClientMessage(err),
		Metadata: err.Metadata,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(err.Kind.HTTPStatus())
	if encErr := json.NewEncoder(w).Encode(resp); encErr != nil {
		// JSON 编码失败时记录日志，响应头已发送无法回滚
		slog.Error("JSON 编码失败", "component", "Errors", "error", encErr)
	}
}

// WriteErrorFrom 将 error 写入 HTTP 响应.
// 非 *Error 类型不暴露内部错误详情，仅返回通用错误消息并记录原始错误.
func WriteErrorFrom(w http.ResponseWriter, err error) {
	e, ok := FromError(err)
	if !ok {
		slog.Error("内部服务器错误", "component", "Errors", "error", err)
		e = internalHTTPError()
	}
	WriteError(w, e)
}

func internalHTTPError() *Error {
	return NewWithKind(900500, "internal", "内部服务器错误", KindInternal)
}

func httpClientMessage(err *Error) string {
	if err.Code >= 50000 && err.Code < 70000 {
		return "内部服务器错误"
	}
	return err.Message
}
