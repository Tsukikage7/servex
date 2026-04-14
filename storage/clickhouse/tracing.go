package clickhouse

import (
	"context"
	"strings"

	driver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "github.com/Tsukikage7/servex/v2/storage/clickhouse"

// tracingClient 为 chClient 添加 OpenTelemetry 链路追踪.
type tracingClient struct {
	inner  *chClient
	tracer trace.Tracer
}

func newTracingClient(inner *chClient) *tracingClient {
	return &tracingClient{
		inner:  inner,
		tracer: otel.Tracer(tracerName),
	}
}

// spanName 从 SQL 语句中提取操作名（如 SELECT / INSERT / CREATE TABLE 等）.
func spanName(query string) string {
	q := strings.TrimSpace(query)
	// 取第一个词作为操作类型
	op := q
	if idx := strings.IndexByte(q, ' '); idx > 0 {
		op = q[:idx]
	}
	return "CH " + strings.ToUpper(op)
}

func (c *tracingClient) startSpan(ctx context.Context, query string) (context.Context, trace.Span) {
	return c.tracer.Start(ctx, spanName(query),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemKey.String("clickhouse"),
			semconv.DBStatement(query),
		),
	)
}

func endSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}

func (c *tracingClient) Exec(ctx context.Context, query string, args ...any) error {
	ctx, span := c.startSpan(ctx, query)

	err := c.inner.Exec(ctx, query, args...)
	endSpan(span, err)
	return err
}

func (c *tracingClient) Query(ctx context.Context, query string, args ...any) (driver.Rows, error) {
	ctx, span := c.startSpan(ctx, query)

	rows, err := c.inner.Query(ctx, query, args...)
	endSpan(span, err)
	return rows, err
}

func (c *tracingClient) QueryRow(ctx context.Context, query string, args ...any) driver.Row {
	ctx, span := c.startSpan(ctx, query)

	row := c.inner.QueryRow(ctx, query, args...)
	endSpan(span, row.Err())
	return row
}

func (c *tracingClient) Select(ctx context.Context, dest any, query string, args ...any) error {
	ctx, span := c.startSpan(ctx, query)

	err := c.inner.Select(ctx, dest, query, args...)
	endSpan(span, err)
	return err
}

func (c *tracingClient) PrepareBatch(ctx context.Context, query string) (driver.Batch, error) {
	ctx, span := c.startSpan(ctx, query)

	batch, err := c.inner.PrepareBatch(ctx, query)
	endSpan(span, err)
	return batch, err
}

func (c *tracingClient) Ping(ctx context.Context) error {
	ctx, span := c.tracer.Start(ctx, "CH PING",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemKey.String("clickhouse"),
			attribute.String("db.operation", "PING"),
		),
	)

	err := c.inner.Ping(ctx)
	endSpan(span, err)
	return err
}

func (c *tracingClient) Close() error {
	return c.inner.Close()
}

func (c *tracingClient) Conn() driver.Conn {
	return c.inner.Conn()
}
