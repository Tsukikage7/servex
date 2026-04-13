package grpcserver

import (
	"context"

	"github.com/Tsukikage7/servex/v2/endpoint"
)

// PassthroughDecode 是 gRPC 直通解码器，将 proto 消息原样传递给 Endpoint.
//
// 适用于 Endpoint 直接操作 proto 消息的场景，无需中间转换。
func PassthroughDecode(_ context.Context, req any) (any, error) {
	return req, nil
}

// PassthroughEncode 是 gRPC 直通编码器，将响应原样返回给调用方.
func PassthroughEncode(_ context.Context, resp any) (any, error) {
	return resp, nil
}

// PassthroughHandler 创建直通编解码的 gRPC Handler.
//
// 等价于 NewEndpointHandler(e, PassthroughDecode, PassthroughEncode, opts...)，
// 适用于 Endpoint 直接接受和返回 proto 消息的场景。
//
// 示例：
//
//	handler := grpcserver.PassthroughHandler(
//	    func(ctx context.Context, req any) (any, error) {
//	        r := req.(*pb.GetUserRequest)
//	        user, err := svc.GetUser(ctx, r.Id)
//	        if err != nil {
//	            return nil, err
//	        }
//	        return &pb.GetUserResponse{Id: user.Id, Name: user.Name}, nil
//	    },
//	    grpcserver.WithResponse(),
//	)
func PassthroughHandler(e endpoint.Endpoint, opts ...EndpointOption) *EndpointHandler {
	return NewEndpointHandler(e, PassthroughDecode, PassthroughEncode, opts...)
}
