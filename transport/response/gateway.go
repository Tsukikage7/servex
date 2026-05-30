package response

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc/status"
)

// GatewayErrorHandler 统一 gateway 错误处理器。
//
// 将 gRPC 错误转换为统一的 JSON 响应格式，支持 i18n读取 Accept-Language 头。
// 与 httpserver 的 responseErrorEncoder 行为一致。
//
// 细粒度 Code 保留：GRPCStatus 将业务 Code 以 JSON 嵌入 gRPC status message，
// 此处理器通过 FromGRPCStatus 从中还原完整 Code如 30002，
// 不会因粗粒度 gRPC code 反向映射被降级如被错误还原为 30001。
func GatewayErrorHandler(
	_ context.Context,
	_ *runtime.ServeMux,
	_ runtime.Marshaler,
	w http.ResponseWriter,
	r *http.Request,
	err error,
) {
	s, _ := status.FromError(err)
	code := FromGRPCStatus(s)

	langs := acceptLanguages(r)
	message := LocalizedMessage(code.ToError(), langs...)

	resp := Response[any]{
		Code:    code.Num,
		Message: message,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code.HTTPStatus())
	_ = json.NewEncoder(w).Encode(resp)
}

// GatewayServeMuxOption 返回注册了统一错误处理器的 ServeMux 选项。
func GatewayServeMuxOption() runtime.ServeMuxOption {
	return runtime.WithErrorHandler(GatewayErrorHandler)
}

// GatewaySuccessResponseMiddleware 返回一个 HTTP middleware，
// 将 gRPC-Gateway 成功响应HTTP 200包裹为统一格式：
//
//	{"code": 0, "message": "成功", "data": <原始 proto JSON>}
//
// 在 gateway server 的 HTTP handler 外层应用，与 GatewayErrorHandler 配合
// 实现成功与错误响应格式的完全统一。
func GatewaySuccessResponseMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &captureResponseWriter{ResponseWriter: w, request: r}
		next.ServeHTTP(rw, r)
		rw.flush()
	})
}

// captureResponseWriter 捕获 gRPC-Gateway 的写入，成功时包裹统一响应体。
type captureResponseWriter struct {
	http.ResponseWriter
	request *http.Request
	buf     bytes.Buffer
	status  int
}

func (c *captureResponseWriter) WriteHeader(code int) {
	c.status = code
}

func (c *captureResponseWriter) Write(b []byte) (int, error) {
	return c.buf.Write(b)
}

// Flush 实现 http.Flusher，防止中间件链丢失 flush 能力。
func (c *captureResponseWriter) Flush() {
	if f, ok := c.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (c *captureResponseWriter) flush() {
	statusCode := c.status
	if statusCode == 0 {
		statusCode = http.StatusOK
	}

	body := c.buf.Bytes()

	if statusCode == http.StatusOK && json.Valid(body) {
		wrapped, err := json.Marshal(struct {
			Code    int             `json:"code"`
			Message string          `json:"message"`
			Data    json.RawMessage `json:"data"`
		}{
			Code:    CodeSuccess.Num,
			Message: LocalizedMessage(nil, acceptLanguages(c.request)...),
			Data:    json.RawMessage(body),
		})
		if err == nil {
			c.ResponseWriter.Header().Set("Content-Type", "application/json; charset=utf-8")
			c.ResponseWriter.WriteHeader(statusCode)
			c.ResponseWriter.Write(wrapped) //nolint:errcheck
			return
		}
	}

	// 非 200 或序列化失败直接透传错误已由 GatewayErrorHandler 格式化
	c.ResponseWriter.WriteHeader(statusCode)
	c.ResponseWriter.Write(body) //nolint:errcheck
}

// acceptLanguages 从请求头解析语言偏好列表。
func acceptLanguages(r *http.Request) []string {
	if r == nil {
		return nil
	}
	if al := r.Header.Get("Accept-Language"); al != "" {
		return []string{al}
	}
	return nil
}
