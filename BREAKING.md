# Breaking Changes

This document records intentional breaking changes for the current v2 development line.

## Unreleased v2

servex v2 narrows the public API surface around stable microservice and AI application infrastructure. Deprecated wrappers, historical aliases, and compatibility-only parameters are removed instead of being carried forward.

### Auth and JWT

- Removed `auth/jwt.NewJWT`.
  - Use `jwt.New(opts...)` when callers should handle configuration errors.
  - Use `jwt.MustNew(opts...)` only for startup-time fail-fast wiring.
- Moved JWT gRPC middleware usage to `auth/jwt/grpcx`.

### Gateway

- Removed fine-grained gateway cross-cutting options:
  - `WithTrace`
  - `WithResponse`
  - `WithRecovery`
  - `WithAuth`
  - `WithCORS`
  - `WithRateLimit`
  - `WithMetrics`
  - `WithLogging`
  - `WithTenant`
  - `WithClientIP`
  - `WithReadinessChecker`
  - `WithLivenessChecker`
- Use `gateway.WithObservability(gateway.ObservabilityConfig{...})` for tracing, metrics, and request logging.
- Use `gateway.WithRuntime(gateway.RuntimeConfig{...})` for response wrapping and panic recovery.
- Use `gateway.WithSecurity(gateway.SecurityConfig{...})` for auth, CORS, rate limit, client IP, and tenant resolution.
- Use `gateway.WithHealthOptions(health.WithReadinessChecker(...), health.WithLivenessChecker(...))` for health checks.

### Collections

- Removed `priorityqueue.ToSlice`.
  - Use `DrainToSlice` when destructive draining is intended.
- Changed `delayqueue.New[T](capacity)` to `delayqueue.New[T]()`.
  - The `capacity` argument was unused and is no longer accepted.

### CLI

- Removed `servex gen aggregate`.
  - Use `servex add aggregate` for DDD aggregate generation.
- Removed support for writing generated service code into historical `services/<name>-service` directories.
  - New generated code targets `services/<name>` only.
- Generated application assembly now uses `cmd/server/app.go` and `newXxx` constructors instead of `cmd/server/provider.go` and `provideXxx` functions.

### OpenAPI

- Removed the OpenAPI builder-level deprecated marker:
  - `Operation.IsDeprecated`
  - `Builder.Deprecated(bool)`
  - `OperationSpec.Deprecated`

servex no longer models deprecated API operations in its OpenAPI builder. If an application needs to publish deprecated OpenAPI operations, keep that policy in application-owned OpenAPI generation code.

### Test Utilities

- Changed `testx/container.Container.Close(ctx)` to `Close()`.
  - The `context.Context` argument was unused.

### Redis

- Removed `storage/redis.NewUniversalClientWithCleanup`.
  - Use `redis.NewClient`, call `Underlying()` when a third-party library needs `*redis.Client`, and close the servex client through `Close()`.

### Observability

- Removed `observability.Runtime`, `NewRuntime`, and `MustNewRuntime`.
- Use explicit constructors instead:
  - `observability.NewLogger`
  - `observability.NewResource`
  - `observability.NewPropagator`
  - `observability.NewTracerProvider`
  - `observability.NewMeterProvider`
  - `observability.InstallGlobal`
- Use `Config.TraceEnabled(name)` and `Config.MetricsEnabled(name)` for component-level enablement checks.

### Rate Limiting

- `middleware/ratelimit.DistributedConfig.FailOpen` changed from `*bool` to `bool`.
- The default is now fail-closed. Set `FailOpen: true` explicitly when backend errors should allow requests through.

### Removed Compatibility Error Sentinels

- Removed `transport/httpclient.ErrMissingLogger`.
- Removed `transport/grpcclient.ErrMissingLogger`.

These sentinels were no longer returned by default construction paths.

## Migration Pattern

Prefer this style for current v2 code:

```go
jwtSvc, err := jwt.New(
    jwt.WithSecretKey("your-secret-key-at-least-32-bytes"),
    jwt.WithLogger(log),
)
if err != nil {
    return err
}

srv := gateway.New(
    gateway.WithLogger(log),
    gateway.WithObservability(gateway.ObservabilityConfig{
        Logging: true,
    }),
    gateway.WithRuntime(gateway.RuntimeConfig{
        Recovery: true,
        Response: true,
    }),
    gateway.WithSecurity(gateway.SecurityConfig{
        Authenticator: jwt.NewAuthenticator(jwtSvc),
        AuthOptions:   []auth.Option{auth.WithAuthorizer(authorizer)},
    }),
)
```
