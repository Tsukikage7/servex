package pubsub

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// TracingPublisher 包装 Publisher，在 Publish 时自动将当前 span context 注入消息 Headers.
type TracingPublisher struct {
	inner Publisher
}

// NewTracingPublisher 创建 TracingPublisher.
func NewTracingPublisher(p Publisher) *TracingPublisher {
	return &TracingPublisher{inner: p}
}

// Publish 将 ctx 中的 trace context 注入每条消息的 Headers，然后委托给内部 Publisher 发布.
func (p *TracingPublisher) Publish(ctx context.Context, topic string, msgs ...*Message) error {
	prop := otel.GetTextMapPropagator()
	for _, msg := range msgs {
		if msg.Headers == nil {
			msg.Headers = make(map[string]string)
		}
		prop.Inject(ctx, propagation.MapCarrier(msg.Headers))
	}
	return p.inner.Publish(ctx, topic, msgs...)
}

// Close 关闭内部 Publisher.
func (p *TracingPublisher) Close() error { return p.inner.Close() }

// ExtractTraceContext 从消息 Headers 提取 trace context，创建 consumer span 并返回.
// 调用方负责在处理完成后调用 span.End().
func ExtractTraceContext(ctx context.Context, msg *Message, tracerName, spanName string) (context.Context, trace.Span) {
	if msg.Headers != nil {
		ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(msg.Headers))
	}
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindConsumer))
	return ctx, span
}

// ExtractTraceContextWithError 与 ExtractTraceContext 相同，额外在 err 非 nil 时记录错误并标记 span 状态.
// 适用于 defer 模式：
//
//	ctx, span := pubsub.ExtractTraceContext(ctx, msg, "svc", "op")
//	defer func() { pubsub.RecordSpanError(span, err) }()
func RecordSpanError(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}
