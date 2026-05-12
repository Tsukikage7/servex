package idempotency

import (
	"context"
	"encoding/json"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// UnaryServerInterceptor 返回 gRPC 一元服务器幂等性拦截器.
// 当请求携带 x-idempotency-key 元数据时，拦截器会：
//  1. 检查该键是否已有结果
//  2. 如果有，直接返回之前的结果
//  3. 如果没有，执行请求并保存结果
//
// 示例:
//
//	store := idempotency.NewRedisStore(redisClient)
//	srv := grpc.NewServer(
//	    grpc.ChainUnaryInterceptor(
//	        idempotency.UnaryServerInterceptor(store),
//	    ),
//	)
func UnaryServerInterceptor(store Store, opts ...Option) grpc.UnaryServerInterceptor {
	if store == nil {
		panic("idempotency: 存储实例不能为空")
	}

	o := applyOptions(store, opts)

	// 默认从元数据提取
	if o.keyExtractor == nil {
		o.keyExtractor = func(ctx any) string {
			if c, ok := ctx.(context.Context); ok {
				if md, ok := metadata.FromIncomingContext(c); ok {
					if vals := md.Get(DefaultGRPCKeyMetadata); len(vals) > 0 {
						return vals[0]
					}
				}
			}
			return ""
		}
	}

	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		// 提取幂等键
		key := o.keyExtractor(ctx)
		if key == "" {
			return handler(ctx, req)
		}

		// 检查是否已有结果
		result, err := store.Get(ctx, key)
		if err != nil {
			if o.skipOnError {
				if o.logger != nil {
					logger.FromContextOr(ctx, o.logger).Warn(
						"[Idempotency] 存储获取失败，跳过检查",
						logger.String("key", key),
						logger.String("method", info.FullMethod),
						logger.Err(err),
					)
				}
				return handler(ctx, req)
			}
			return nil, status.Error(codes.Internal, "idempotency check failed")
		}

		if result != nil {
			// 返回之前的结果
			if o.logger != nil {
				logger.FromContextOr(ctx, o.logger).Debug(
					"[Idempotency] 缓存命中",
					logger.String("key", key),
					logger.String("method", info.FullMethod),
				)
			}
			return decodeGRPCResult(result)
		}

		// 尝试获取处理锁
		locked, err := store.SetNX(ctx, key, o.lockTimeout)
		if err != nil {
			if o.skipOnError {
				if o.logger != nil {
					logger.FromContextOr(ctx, o.logger).Warn(
						"[Idempotency] 获取锁失败，跳过检查",
						logger.String("key", key),
						logger.String("method", info.FullMethod),
						logger.Err(err),
					)
				}
				return handler(ctx, req)
			}
			return nil, status.Error(codes.Internal, "idempotency lock failed")
		}

		if !locked {
			// 请求正在处理中
			return nil, status.Error(codes.Aborted, "request in progress")
		}

		// 执行请求
		resp, err := handler(ctx, req)

		// 保存结果
		saveResult := &Result{
			CreatedAt: time.Now(),
		}
		if err != nil {
			st, _ := status.FromError(err)
			saveResult.StatusCode = int(st.Code())
			saveResult.Error = st.Message()
		} else {
			saveResult.StatusCode = int(codes.OK)
			saveResult.Body, _ = json.Marshal(resp)
		}

		if saveErr := store.Set(ctx, key, saveResult, o.ttl); saveErr != nil {
			if o.logger != nil {
				logger.FromContextOr(ctx, o.logger).Warn(
					"[Idempotency] 存储写入失败",
					logger.String("key", key),
					logger.String("method", info.FullMethod),
					logger.Err(saveErr),
				)
			}
		}

		return resp, err
	}
}

// decodeGRPCResult 解码 gRPC 结果.
func decodeGRPCResult(result *Result) (any, error) {
	if result.Error != "" {
		return nil, status.Error(codes.Code(result.StatusCode), result.Error)
	}
	// 返回原始 JSON 数据
	return result.Body, nil
}
