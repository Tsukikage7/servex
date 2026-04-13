package middleware

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/Tsukikage7/servex/v2/domain/cqrs"
)

const tracerName = "servex/cqrs"

// CommandTracing 为命令处理器添加链路追踪装饰器.
func CommandTracing[C, R any](spanName string, tracer ...trace.Tracer) cqrs.CommandMiddleware[C, R] {
	t := resolveTracer(tracer)
	return func(next cqrs.CommandHandler[C, R]) cqrs.CommandHandler[C, R] {
		return &commandTracingHandler[C, R]{
			next:     next,
			tracer:   t,
			spanName: spanName,
		}
	}
}

type commandTracingHandler[C, R any] struct {
	next     cqrs.CommandHandler[C, R]
	tracer   trace.Tracer
	spanName string
}

func (h *commandTracingHandler[C, R]) Handle(ctx context.Context, cmd C) (C, R, error) {
	ctx, span := h.tracer.Start(ctx, h.spanName,
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	c, r, err := h.next.Handle(ctx, cmd)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return c, r, err
}

// QueryTracing 为查询处理器添加链路追踪装饰器.
func QueryTracing[Q, R any](spanName string, tracer ...trace.Tracer) cqrs.QueryMiddleware[Q, R] {
	t := resolveTracer(tracer)
	return func(next cqrs.QueryHandler[Q, R]) cqrs.QueryHandler[Q, R] {
		return &queryTracingHandler[Q, R]{
			next:     next,
			tracer:   t,
			spanName: spanName,
		}
	}
}

type queryTracingHandler[Q, R any] struct {
	next     cqrs.QueryHandler[Q, R]
	tracer   trace.Tracer
	spanName string
}

func (h *queryTracingHandler[Q, R]) Handle(ctx context.Context, query Q) (R, error) {
	ctx, span := h.tracer.Start(ctx, h.spanName,
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	r, err := h.next.Handle(ctx, query)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return r, err
}

// resolveTracer 从可选参数中获取 tracer，若未传则使用全局 tracer.
func resolveTracer(tracers []trace.Tracer) trace.Tracer {
	if len(tracers) > 0 && tracers[0] != nil {
		return tracers[0]
	}
	return otel.Tracer(tracerName)
}
