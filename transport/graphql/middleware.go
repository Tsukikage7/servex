package graphql

import (
	"fmt"
	"time"

	gql "github.com/graphql-go/graphql"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// ResolveFunc 字段解析函数类型别名.
type ResolveFunc = gql.FieldResolveFn

// Middleware GraphQL resolve 层中间件.
type Middleware func(ResolveFunc) ResolveFunc

// ChainMiddleware 链接多个中间件，outer 最先执行.
func ChainMiddleware(outer Middleware, others ...Middleware) Middleware {
	return func(next ResolveFunc) ResolveFunc {
		for i := len(others) - 1; i >= 0; i-- {
			next = others[i](next)
		}
		return outer(next)
	}
}

// WrapResolve 将中间件应用到单个 resolve 函数.
// 中间件按声明顺序包裹，第一个中间件最先执行.
func WrapResolve(fn gql.FieldResolveFn, mw ...Middleware) gql.FieldResolveFn {
	for i := len(mw) - 1; i >= 0; i-- {
		fn = mw[i](fn)
	}
	return fn
}

// LoggingMiddleware 记录 resolve 执行耗时.
func LoggingMiddleware(log logger.Logger) Middleware {
	return func(next ResolveFunc) ResolveFunc {
		return func(p gql.ResolveParams) (any, error) {
			start := time.Now()
			result, err := next(p)
			elapsed := time.Since(start)

			fieldName := p.Info.FieldName
			if err != nil {
				log.With(
					logger.Field{Key: "field", Value: fieldName},
					logger.Field{Key: "duration_ms", Value: elapsed.Milliseconds()},
					logger.Field{Key: "error", Value: err.Error()},
				).Error("[GraphQL] 解析器执行失败")
			} else {
				log.With(
					logger.Field{Key: "field", Value: fieldName},
					logger.Field{Key: "duration_ms", Value: elapsed.Milliseconds()},
				).Debug("[GraphQL] 解析器执行完成")
			}
			return result, err
		}
	}
}

// RecoveryMiddleware panic 恢复中间件，捕获 resolve 中的 panic 并返回错误.
func RecoveryMiddleware(log logger.Logger) Middleware {
	return func(next ResolveFunc) ResolveFunc {
		return func(p gql.ResolveParams) (result any, err error) {
			defer func() {
				if r := recover(); r != nil {
					log.With(
						logger.Field{Key: "field", Value: p.Info.FieldName},
						logger.Field{Key: "panic", Value: fmt.Sprintf("%v", r)},
					).Error("[GraphQL] 解析器异常已恢复")
					err = ErrInternalError
				}
			}()
			return next(p)
		}
	}
}

// TracingMiddleware OpenTelemetry 链路追踪中间件，为每次 resolve 创建 span.
func TracingMiddleware(serviceName string) Middleware {
	return func(next ResolveFunc) ResolveFunc {
		return func(p gql.ResolveParams) (any, error) {
			ctx := p.Context
			tracer := otel.Tracer(serviceName)
			spanName := "graphql.resolve." + p.Info.FieldName
			ctx, span := tracer.Start(ctx, spanName)
			defer span.End()

			span.SetAttributes(
				attribute.String("graphql.field", p.Info.FieldName),
			)

			// 将带 span 的 context 传递给下游
			p.Context = ctx
			result, err := next(p)
			if err != nil {
				span.SetAttributes(attribute.String("graphql.error", err.Error()))
			}
			return result, err
		}
	}
}
