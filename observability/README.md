# observability

`observability` 提供服务级可观测性配置结构，用于统一描述 logger、tracing、metrics 和组件埋点开关。

它允许服务只启用日志，不强制接入 trace、metrics 或外部 collector。日志由 `observability/logger` 创建；OpenTelemetry provider 建议直接使用官方 SDK 或具体组件包装。

## 配置示例

```yaml
observability:
  service:
    name: order
    version: 1.0.0
    environment: local
  logger:
    level: info
    format: console
    output: console
  tracing:
    enabled: false
    sampling_rate: 1
    exporters: []
  metrics:
    enabled: false
    interval: 60s
    exporters: []
  instrumentations:
    default_enabled: true
    overrides:
      redis: false
      websocket: true
      goim: true
```

只通过日志暴露服务状态时，保持 `tracing.enabled=false` 和 `metrics.enabled=false` 即可。组件埋点开关只决定框架是否允许某个组件接入埋点，最终 trace 和 metrics 仍受各自的 `enabled` 控制。

## OTLP 示例

```yaml
observability:
  service:
    name: order
    version: 1.0.0
    environment: prod
  logger:
    level: info
    format: json
    output: console
  tracing:
    enabled: true
    sampling_rate: 0.1
    exporters:
      - type: otlp
        protocol: grpc
        endpoint: otel-collector:4317
        insecure: true
  metrics:
    enabled: true
    interval: 30s
    exporters:
      - type: otlp
        protocol: http
        endpoint: otel-collector:4318
        insecure: true
  instrumentations:
    default_enabled: true
    overrides:
      redis: true
      rdbms: true
      goim: true
```

## 装配用法

CLI 模板默认只创建日志，不强制接入 trace、metrics 或外部 collector：

```go
cfg.Observability.Service.Name = cfg.Name
cfg.Observability.Service.Version = cfg.Version
cfg.Observability.ApplyDefaults()

log, err := logger.NewLogger(&cfg.Observability.Logger)
if err != nil {
	return nil, err
}
```

需要启用 OpenTelemetry 时，使用官方 SDK 显式装配 provider，再按需设置全局 provider：

```go
otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
	propagation.TraceContext{},
	propagation.Baggage{},
))
otel.SetTracerProvider(tracerProvider)
otel.SetMeterProvider(meterProvider)
```
