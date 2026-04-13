package idempotency

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Tsukikage7/servex/v2/endpoint"
	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// IdempotentRequest 支持幂等性的请求接口.
// 请求类型可以实现此接口来提供幂等键.
type IdempotentRequest interface {
	IdempotencyKey() string
}

// EndpointMiddleware 返回 Endpoint 幂等性中间件.
// 当请求携带幂等键时，中间件会：
//  1. 检查该键是否已有结果
//  2. 如果有，直接返回之前的结果
//  3. 如果没有，执行请求并保存结果
//
// 请求类型需要实现 IdempotentRequest 接口，或通过 WithKeyExtractor 自定义提取逻辑.
// 示例:
//
//	store := idempotency.NewRedisStore(redisClient)
//	endpoint = idempotency.EndpointMiddleware(store)(endpoint)
func EndpointMiddleware(store Store, opts ...Option) endpoint.Middleware {
	if store == nil {
		panic("idempotency: 存储实例不能为空")
	}

	o := applyOptions(store, opts)

	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request any) (any, error) {
			// 提取幂等键
			var key string
			if o.keyExtractor != nil {
				key = o.keyExtractor(request)
			} else if ir, ok := request.(IdempotentRequest); ok {
				key = ir.IdempotencyKey()
			}

			// 没有幂等键，直接执行
			if key == "" {
				return next(ctx, request)
			}

			// 检查是否已有结果
			result, err := store.Get(ctx, key)
			if err != nil {
				if o.skipOnError {
					if o.logger != nil {
						logger.FromContext(ctx).Warn(
							"[Idempotency] 存储获取失败，跳过检查",
							logger.String("key", key),
							logger.Err(err),
						)
					}
					return next(ctx, request)
				}
				return nil, ErrStoreFailure
			}

			if result != nil {
				// 返回之前的结果
				if o.logger != nil {
					logger.FromContext(ctx).Debug(
						"[Idempotency] 缓存命中",
						logger.String("key", key),
					)
				}
				return decodeEndpointResult(result)
			}

			// 尝试获取处理锁
			locked, err := store.SetNX(ctx, key, o.lockTimeout)
			if err != nil {
				if o.skipOnError {
					if o.logger != nil {
						logger.FromContext(ctx).Warn(
							"[Idempotency] 获取锁失败，跳过检查",
							logger.String("key", key),
							logger.Err(err),
						)
					}
					return next(ctx, request)
				}
				return nil, ErrStoreFailure
			}

			if !locked {
				// 请求正在处理中
				return nil, ErrRequestInProgress
			}

			// 执行请求
			resp, err := next(ctx, request)

			// 保存结果
			saveResult := &Result{
				CreatedAt: time.Now(),
			}
			if err != nil {
				saveResult.Error = err.Error()
			} else {
				saveResult.Body, _ = json.Marshal(resp)
			}

			if saveErr := store.Set(ctx, key, saveResult, o.ttl); saveErr != nil {
				if o.logger != nil {
					logger.FromContext(ctx).Warn(
						"[Idempotency] 存储写入失败",
						logger.String("key", key),
						logger.Err(saveErr),
					)
				}
			}

			return resp, err
		}
	}
}

// decodeEndpointResult 解码 Endpoint 结果.
func decodeEndpointResult(result *Result) (any, error) {
	if result.Error != "" {
		return nil, &idempotencyError{msg: result.Error}
	}
	// 返回原始 JSON 数据，调用方需要自行解析
	return result.Body, nil
}

// idempotencyError 幂等性返回的错误.
type idempotencyError struct {
	msg string
}

func (e *idempotencyError) Error() string {
	return e.msg
}
