package httpserver

import (
	"context"
	"net/http"

	"github.com/Tsukikage7/servex/v2/endpoint"
	"github.com/Tsukikage7/servex/v2/transport/response"
)

// Handle 创建类型安全的 HTTP Handler，自动处理内容协商编解码与统一响应格式.
//
// 适用于请求参数从请求体读取的场景POST/PUT/PATCH。
// 成功响应自动包装为 {"code":0,"message":"成功","data":{...}}；
// 若 Resp 已是 response.Response[T] 或 response.PagedResponse[T]，则不再二次包装。
// 错误自动转换为 {"code":xxxxx,"message":"..."} 并映射正确 HTTP 状态码。
//
// 示例：
//
//	router.POST("/users", httpserver.Handle(
//	    func(ctx context.Context, req CreateUserReq) (*UserResp, error) {
//	        return svc.CreateUser(ctx, req)
//	        // 返回：{"code":0,"message":"成功","data":{"id":1,"name":"Alice"}}
//	    },
//	))
func Handle[Req any, Resp any](
	fn func(ctx context.Context, req Req) (Resp, error),
	opts ...EndpointOption,
) http.Handler {
	ep := endpoint.Endpoint(func(ctx context.Context, req any) (any, error) {
		resp, err := fn(ctx, req.(Req))
		if err != nil {
			return nil, err
		}
		return wrapEnvelope(resp), nil
	})
	return NewEndpointHandler(
		ep,
		DecodeCodecRequest[Req](),
		EncodeCodecResponse,
		append([]EndpointOption{
			WithBefore(WithRequest()),
			WithResponse(),
			WithValidate(),
		}, opts...)...,
	)
}

// HandleWith 创建带自定义解码器的类型安全 HTTP Handler.
//
// 适用于需要从路径参数、查询字符串等位置提取请求数据的场景GET/DELETE。
// 响应包装规则与 Handle 相同。
//
// 示例：
//
//	router.GET("/users/{id}", httpserver.HandleWith(
//	    func(ctx context.Context, r *http.Request) (GetUserReq, error) {
//	        return GetUserReq{ID: r.PathValue("id")}, nil
//	    },
//	    func(ctx context.Context, req GetUserReq) (*UserResp, error) {
//	        return svc.GetUser(ctx, req.ID)
//	        // 返回：{"code":0,"message":"成功","data":{"id":"u123","name":"Alice"}}
//	    },
//	))
func HandleWith[Req any, Resp any](
	decode func(ctx context.Context, r *http.Request) (Req, error),
	fn func(ctx context.Context, req Req) (Resp, error),
	opts ...EndpointOption,
) http.Handler {
	ep := endpoint.Endpoint(func(ctx context.Context, req any) (any, error) {
		resp, err := fn(ctx, req.(Req))
		if err != nil {
			return nil, err
		}
		return wrapEnvelope(resp), nil
	})
	dec := func(ctx context.Context, r *http.Request) (any, error) {
		return decode(ctx, r)
	}
	return NewEndpointHandler(
		ep,
		dec,
		EncodeCodecResponse,
		append([]EndpointOption{
			WithBefore(WithRequest()),
			WithResponse(),
			WithValidate(),
		}, opts...)...,
	)
}

// wrapEnvelope 将响应包装在统一响应体中.
//
// 若 v 已实现 response.Envelope即已是 Response[T] 或 PagedResponse[T]，
// 直接返回，不做二次包装.
func wrapEnvelope(v any) any {
	if _, ok := v.(response.Envelope); ok {
		return v
	}
	return response.OK(v)
}
