# observability

`observability` 是 Servex 的服务级可观测性入口，用于统一创建 logger、OpenTelemetry tracer provider、meter provider 和 trace context propagator。

它允许服务只启用日志，不强制接入 trace、metrics 或外部 collector；也允许同一服务同时配置多个 exporter。

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

## Wire 用法

CLI 模板会生成 `provideObservability`，再从 `*observability.Runtime` 派生 `logger.Logger`：

```go
rt, err := observability.NewRuntime(context.Background(), cfg.Observability)
log := rt.Logger()
```

应用退出时调用 `rt.Shutdown(ctx)`，统一刷新 trace、metrics 和日志资源。

