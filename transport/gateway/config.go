package gateway

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/observability/logger"
	"github.com/Tsukikage7/servex/v2/observability/metrics"
	"github.com/Tsukikage7/servex/v2/transport"
)

// NewFromConfig 从配置创建 Gateway。
//
// Config 只描述可序列化的服务行为；运行时对象认证器、限流器、租户解析器、
// 自定义中间件等继续通过 additionalOpts 由 Wire 注入。
func NewFromConfig(cfg *transport.GatewayConfig, log logger.Logger, additionalOpts ...Option) (*Server, error) {
	if cfg == nil {
		return nil, fmt.Errorf("gateway: 配置不能为空")
	}

	opts := []Option{
		WithLogger(log),
		WithConfig(*cfg),
	}

	if cfg.Metrics.Enabled {
		collector, err := metrics.NewMetrics(&metrics.Config{
			Path:        cfg.Metrics.Path,
			Namespace:   cfg.Metrics.Namespace,
			ServiceName: cfg.Metrics.ServiceName,
			Version:     cfg.Version,
		})
		if err != nil {
			return nil, fmt.Errorf("gateway: 创建指标收集器失败: %w", err)
		}
		opts = append(opts, WithObservability(ObservabilityConfig{Metrics: collector}))
	}

	opts = append(opts, additionalOpts...)
	return New(opts...), nil
}
