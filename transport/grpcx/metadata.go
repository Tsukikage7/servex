package grpcx

import (
	"context"

	"google.golang.org/grpc/metadata"
)

// GetMetadataValue 从 gRPC metadata 中获取单个值.
// 若 key 不存在或无值，返回空字符串.
func GetMetadataValue(ctx context.Context, key string) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	values := md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// GetMetadataValues 从 gRPC metadata 中获取多个值.
// 若 key 不存在，返回 nil.
func GetMetadataValues(ctx context.Context, key string) []string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil
	}
	return md.Get(key)
}

// SetOutgoingMetadata 设置出站 metadata客户端调用前设置.
// 会替换已有的出站 metadata.
func SetOutgoingMetadata(ctx context.Context, kv ...string) context.Context {
	return metadata.NewOutgoingContext(ctx, metadata.Pairs(kv...))
}

// AppendOutgoingMetadata 追加出站 metadata.
// 在已有出站 metadata 基础上追加新的键值对.
func AppendOutgoingMetadata(ctx context.Context, kv ...string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, kv...)
}

// CopyIncomingToOutgoing 将入站 metadata 复制到出站用于中间代理/网关.
// 若指定 keys，则只复制指定的 key；若未指定则复制全部.
func CopyIncomingToOutgoing(ctx context.Context, keys ...string) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}

	if len(keys) == 0 {
		// 复制全部入站 metadata
		return metadata.NewOutgoingContext(ctx, md.Copy())
	}

	// 只复制指定的 key
	pairs := make([]string, 0, len(keys)*2)
	for _, key := range keys {
		values := md.Get(key)
		for _, v := range values {
			pairs = append(pairs, key, v)
		}
	}
	if len(pairs) == 0 {
		return ctx
	}
	return metadata.NewOutgoingContext(ctx, metadata.Pairs(pairs...))
}
