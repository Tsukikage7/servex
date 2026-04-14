package neo4j

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "servex/neo4j"

func (c *Client) startSpan(ctx context.Context, op string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	if !c.enableTracing {
		return ctx, trace.SpanFromContext(ctx) // noop span
	}
	allAttrs := append([]attribute.KeyValue{
		attribute.String("db.system", "neo4j"),
		attribute.String("db.name", c.database),
	}, attrs...)
	return otel.Tracer(tracerName).Start(ctx, "NEO4J "+op,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(allAttrs...),
	)
}

func (c *Client) endSpan(span trace.Span, err error) {
	if !c.enableTracing {
		return
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}
