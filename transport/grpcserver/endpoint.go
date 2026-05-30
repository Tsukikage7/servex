package grpcserver

import (
	"context"

	"google.golang.org/grpc/metadata"

	"github.com/Tsukikage7/servex/v2/endpoint"
	"github.com/Tsukikage7/servex/v2/transport/response"
)

// DecodeRequestFunc 从 gRPC 请求解码为业务请求对象.
//
// 示例：
//
//	func decodeGetUserRequest(_ context.Context, req any) (any, error) {
//	    r := req.(*pb.GetUserRequest)
//	    return GetUserRequest{ID: int(r.Id)}, nil
//	}
type DecodeRequestFunc func(ctx context.Context, request any) (any, error)

// EncodeResponseFunc 将业务响应编码为 gRPC 响应.
//
// 示例：
//
//	func encodeGetUserResponse(_ context.Context, resp any) (any, error) {
//	    r := resp.(GetUserResponse)
//	    return &pb.GetUserResponse{
//	        Id:   int64(r.ID),
//	        Name: r.Name,
//	    }, nil
//	}
type EncodeResponseFunc func(ctx context.Context, response any) (any, error)

// RequestFunc 可以从 gRPC metadata 中提取信息并放入 context.
//
// 在 endpoint 调用之前执行.
type RequestFunc func(ctx context.Context, md metadata.MD) context.Context

// ResponseFunc 可以从 context 中提取信息并操作 gRPC metadata.
//
// header 和 trailer 参数允许修改响应的 metadata.
type ResponseFunc func(ctx context.Context, header *metadata.MD, trailer *metadata.MD) context.Context

// Handler gRPC 服务处理器接口.
//
// 实现此接口的类型可以处理 gRPC 请求.
type Handler interface {
	ServeGRPC(ctx context.Context, request any) (context.Context, any, error)
}

// ErrorHandlerFunc 错误处理函数.
type ErrorHandlerFunc func(err error) error

// EndpointHandler 将 Endpoint 包装为 gRPC Handler.
//
// 示例：
//
//	getUserHandler := server.NewEndpointHandler(
//	    getUserEndpoint,
//	    decodeGetUserRequest,
//	    encodeGetUserResponse,
//	)
type EndpointHandler struct {
	endpoint     endpoint.Endpoint
	dec          DecodeRequestFunc
	enc          EncodeResponseFunc
	before       []RequestFunc
	after        []ResponseFunc
	errorHandler ErrorHandlerFunc
}

// EndpointOption EndpointHandler 配置选项.
type EndpointOption func(*EndpointHandler)

// NewEndpointHandler 创建 gRPC EndpointHandler.
//
// 示例：
//
//	handler := server.NewEndpointHandler(
//	    getUserEndpoint,
//	    decodeGetUserRequest,
//	    encodeGetUserResponse,
//	    server.WithBefore(extractAuthFromMD),
//	)
func NewEndpointHandler(
	e endpoint.Endpoint,
	dec DecodeRequestFunc,
	enc EncodeResponseFunc,
	opts ...EndpointOption,
) *EndpointHandler {
	h := &EndpointHandler{
		endpoint: e,
		dec:      dec,
		enc:      enc,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// ServeGRPC 实现 Handler 接口.
//
// 这个方法应该从你的 gRPC 服务实现中调用.
//
// 示例在你的 gRPC 服务实现中：
//
//	func (s *userServer) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
//	    _, resp, err := s.getUserHandler.ServeGRPC(ctx, req)
//	    if err != nil {
//	        return nil, err
//	    }
//	    return resp.(*pb.GetUserResponse), nil
//	}
func (h *EndpointHandler) ServeGRPC(ctx context.Context, request any) (context.Context, any, error) {
	// 提取 metadata 并执行 before 函数
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = metadata.MD{}
	}
	for _, f := range h.before {
		ctx = f(ctx, md)
	}

	// 解码请求
	req, err := h.dec(ctx, request)
	if err != nil {
		return ctx, nil, h.handleError(err)
	}

	// 调用 endpoint
	response, err := h.endpoint(ctx, req)
	if err != nil {
		return ctx, nil, h.handleError(err)
	}

	// 执行 after 函数
	var header, trailer metadata.MD
	for _, f := range h.after {
		ctx = f(ctx, &header, &trailer)
	}

	// 编码响应
	grpcResp, err := h.enc(ctx, response)
	if err != nil {
		return ctx, nil, h.handleError(err)
	}

	return ctx, grpcResp, nil
}

// handleError 处理错误.
func (h *EndpointHandler) handleError(err error) error {
	if h.errorHandler != nil {
		return h.errorHandler(err)
	}
	return err
}

// WithBefore 添加请求前处理函数.
func WithBefore(funcs ...RequestFunc) EndpointOption {
	return func(h *EndpointHandler) {
		h.before = append(h.before, funcs...)
	}
}

// WithAfter 添加响应后处理函数.
func WithAfter(funcs ...ResponseFunc) EndpointOption {
	return func(h *EndpointHandler) {
		h.after = append(h.after, funcs...)
	}
}

// WithErrorHandler 设置错误处理器.
func WithErrorHandler(handler ErrorHandlerFunc) EndpointOption {
	return func(h *EndpointHandler) {
		h.errorHandler = handler
	}
}

// WithResponse 启用统一响应格式的错误处理.
//
// 错误将自动转换为正确的 gRPC 状态码，
// 内部错误5xxxx的详细信息将被隐藏.
func WithResponse() EndpointOption {
	return func(h *EndpointHandler) {
		h.errorHandler = response.GRPCError
	}
}
