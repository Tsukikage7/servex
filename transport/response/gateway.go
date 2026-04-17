package response

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc/status"
)

// GatewayErrorHandler 统一 gateway 错误处理器。
//
// 将 gRPC 错误转换为统一的 JSON 响应格式，支持 i18n（读取 Accept-Language 头）。
// 与 httpserver 的 responseErrorEncoder 行为一致。
//
// 细粒度 Code 保留：GRPCStatus 将业务 Code 以 JSON 嵌入 gRPC status message，
// 此处理器通过 FromGRPCStatus 从中还原完整 Code（如 30002），
// 不会因粗粒度 gRPC code 反向映射被降级（如被错误还原为 30001）。
//
// 用法：
//
//	srv := gateway.New(
//	    gateway.WithServeMuxOptions(response.GatewayServeMuxOption()),
//	)
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
	w.WriteHeader(code.HTTPStatus)
	_ = json.NewEncoder(w).Encode(resp)
}

// GatewayServeMuxOption 返回注册了统一错误处理器的 ServeMux 选项。
//
// 与 gateway.WithUnifiedResponse() 配合使用时无需重复设置。
func GatewayServeMuxOption() runtime.ServeMuxOption {
	return runtime.WithErrorHandler(GatewayErrorHandler)
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
