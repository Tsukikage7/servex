package compose

import (
	"context"
	"fmt"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// OTelCallbackHandler 将 Graph 执行写入 OpenTelemetry Span.
type OTelCallbackHandler struct {
	tracer trace.Tracer
	mu     sync.Mutex
	spans  map[string]trace.Span // RunID/NodeID → span
}

// NewOTelCallbackHandler 创建 OTel 回调处理器.
func NewOTelCallbackHandler(tracer trace.Tracer) *OTelCallbackHandler {
	return &OTelCallbackHandler{tracer: tracer, spans: make(map[string]trace.Span)}
}

// spanKey 用 RunID + NodeID 组合为唯一 key，防止并发执行时 key 冲突.
func (h *OTelCallbackHandler) spanKey(info NodeRunInfo) string {
	return info.RunID + "/" + info.NodeID
}

func (h *OTelCallbackHandler) OnGraphStart(_ context.Context, _ GraphRunInfo, _ any) {
	// Graph 级 Span 由调用方负责，这里不创建新 Span
}

func (h *OTelCallbackHandler) OnGraphEnd(ctx context.Context, _ GraphRunInfo, _ any, err error) {
	span := trace.SpanFromContext(ctx)
	if err != nil {
		span.RecordError(err)
	}
}

func (h *OTelCallbackHandler) OnNodeStart(ctx context.Context, info NodeRunInfo, _ any) {
	_, span := h.tracer.Start(ctx, fmt.Sprintf("compose.node/%s", info.NodeID),
		trace.WithAttributes(
			attribute.String("compose.node_id", info.NodeID),
			attribute.String("compose.node_kind", info.NodeKind),
			attribute.String("compose.run_id", info.RunID),
		),
	)
	h.mu.Lock()
	h.spans[h.spanKey(info)] = span
	h.mu.Unlock()
}

func (h *OTelCallbackHandler) OnNodeEnd(_ context.Context, info NodeRunInfo, _ any, err error) {
	h.mu.Lock()
	key := h.spanKey(info)
	span, ok := h.spans[key]
	if ok {
		delete(h.spans, key)
	}
	h.mu.Unlock()
	if ok {
		if err != nil {
			span.RecordError(err)
		}
		span.End()
	}
}

func (h *OTelCallbackHandler) OnNodeSkip(_ context.Context, _ NodeRunInfo, _ string) {
	// 可选：在父 Span 上记录 skip event，暂不实现
}

var _ CallbackHandler = (*OTelCallbackHandler)(nil)
