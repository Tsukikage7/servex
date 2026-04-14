package jobqueue

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// TracingClient 包装 Client，在 Enqueue 时自动将 trace context 注入 Job Headers.
type TracingClient struct {
	inner Client
}

// NewTracingClient 创建 TracingClient.
func NewTracingClient(c Client) *TracingClient {
	return &TracingClient{inner: c}
}

// Enqueue 将 ctx 中的 trace context 注入 Job Headers，然后委托给内部 Client 投递.
func (c *TracingClient) Enqueue(ctx context.Context, job *Job) error {
	if job.Headers == nil {
		job.Headers = make(map[string]string)
	}
	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(job.Headers))
	return c.inner.Enqueue(ctx, job)
}

// Close 关闭内部 Client.
func (c *TracingClient) Close() error { return c.inner.Close() }

// TracingWorker 包装 Worker，执行 handler 前提取 trace context 并创建 consumer span.
type TracingWorker struct {
	inner      Worker
	tracerName string
}

// NewTracingWorker 创建 TracingWorker.
func NewTracingWorker(w Worker, tracerName string) *TracingWorker {
	return &TracingWorker{inner: w, tracerName: tracerName}
}

// Register 注册任务处理器，自动注入 trace context 包装.
func (w *TracingWorker) Register(jobType string, handler Handler) {
	w.inner.Register(jobType, func(ctx context.Context, job *Job) error {
		if job.Headers != nil {
			ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(job.Headers))
		}
		tracer := otel.Tracer(w.tracerName)
		ctx, span := tracer.Start(ctx, "job/"+jobType, trace.WithSpanKind(trace.SpanKindConsumer))
		defer span.End()

		err := handler(ctx, job)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		return err
	})
}

// Start 启动内部 Worker.
func (w *TracingWorker) Start(ctx context.Context) error { return w.inner.Start(ctx) }

// Close 关闭内部 Worker.
func (w *TracingWorker) Close() error { return w.inner.Close() }
