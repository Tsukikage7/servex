package middleware

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/Tsukikage7/servex/v2/domain/cqrs"
)

// CommandMetrics 为命令处理器添加 Prometheus 指标装饰器.
// 收集命令总次数、成功/失败次数和执行耗时直方图.
func CommandMetrics[C, R any](commandName string, registerer prometheus.Registerer) cqrs.CommandMiddleware[C, R] {
	if registerer == nil {
		registerer = prometheus.DefaultRegisterer
	}

	total := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:        "cqrs_command_total",
		Help:        "命令处理总次数",
		ConstLabels: prometheus.Labels{"command": commandName},
	}, []string{"result"})

	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:        "cqrs_command_duration_seconds",
		Help:        "命令处理耗时",
		ConstLabels: prometheus.Labels{"command": commandName},
		Buckets:     prometheus.DefBuckets,
	}, []string{"result"})

	// 忽略重复注册错误（测试环境多次调用）
	_ = registerer.Register(total)
	_ = registerer.Register(duration)

	return func(next cqrs.CommandHandler[C, R]) cqrs.CommandHandler[C, R] {
		return &commandMetricsHandler[C, R]{
			next:     next,
			total:    total,
			duration: duration,
		}
	}
}

type commandMetricsHandler[C, R any] struct {
	next     cqrs.CommandHandler[C, R]
	total    *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

func (h *commandMetricsHandler[C, R]) Handle(ctx context.Context, cmd C) (C, R, error) {
	start := time.Now()
	c, r, err := h.next.Handle(ctx, cmd)
	elapsed := time.Since(start).Seconds()

	result := "success"
	if err != nil {
		result = "error"
	}

	h.total.WithLabelValues(result).Inc()
	h.duration.WithLabelValues(result).Observe(elapsed)

	return c, r, err
}

// QueryMetrics 为查询处理器添加 Prometheus 指标装饰器.
func QueryMetrics[Q, R any](queryName string, registerer prometheus.Registerer) cqrs.QueryMiddleware[Q, R] {
	if registerer == nil {
		registerer = prometheus.DefaultRegisterer
	}

	total := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:        "cqrs_query_total",
		Help:        "查询处理总次数",
		ConstLabels: prometheus.Labels{"query": queryName},
	}, []string{"result"})

	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:        "cqrs_query_duration_seconds",
		Help:        "查询处理耗时",
		ConstLabels: prometheus.Labels{"query": queryName},
		Buckets:     prometheus.DefBuckets,
	}, []string{"result"})

	_ = registerer.Register(total)
	_ = registerer.Register(duration)

	return func(next cqrs.QueryHandler[Q, R]) cqrs.QueryHandler[Q, R] {
		return &queryMetricsHandler[Q, R]{
			next:     next,
			total:    total,
			duration: duration,
		}
	}
}

type queryMetricsHandler[Q, R any] struct {
	next     cqrs.QueryHandler[Q, R]
	total    *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

func (h *queryMetricsHandler[Q, R]) Handle(ctx context.Context, query Q) (R, error) {
	start := time.Now()
	r, err := h.next.Handle(ctx, query)
	elapsed := time.Since(start).Seconds()

	result := "success"
	if err != nil {
		result = "error"
	}

	h.total.WithLabelValues(result).Inc()
	h.duration.WithLabelValues(result).Observe(elapsed)

	return r, err
}
