# transport/grpcx

`github.com/Tsukikage7/servex/v2/transport/grpcx` — gRPC 工具包，提供跨包复用的 gRPC 辅助类型与函数。

## 功能概览

### Stream 包装

- `WrappedServerStream` — 包装 gRPC ServerStream，允许替换 context
- `WrapServerStream(stream, ctx)` — 创建包装后的 ServerStream

### Metadata 操作

- `GetMetadataValue(ctx, key)` — 从入站 metadata 中获取单个值
- `GetMetadataValues(ctx, key)` — 从入站 metadata 中获取多个值
- `SetOutgoingMetadata(ctx, kv...)` — 设置出站 metadata（替换已有）
- `AppendOutgoingMetadata(ctx, kv...)` — 追加出站 metadata
- `CopyIncomingToOutgoing(ctx, keys...)` — 将入站 metadata 复制到出站（用于代理/网关）

### 错误处理

- `Error(code, msg)` / `Errorf(code, format, args...)` — 创建 gRPC status error
- `Code(err)` — 提取 gRPC status code
- `Message(err)` — 提取 gRPC status message
- `IsCode(err, code)` — 检查 error 是否匹配指定 code
- 便捷构造器：`NotFound`、`InvalidArgument`、`PermissionDenied`、`Unauthenticated`、`Internal`、`Unavailable`、`AlreadyExists`、`DeadlineExceeded`

### 健康检查

- `HealthCheck(ctx, conn)` — 标准 gRPC 健康检查
- `WaitForReady(ctx, conn, timeout)` — 等待连接就绪

## 使用示例

### Stream 包装

```go
import "github.com/Tsukikage7/servex/v2/transport/grpcx"

func myStreamInterceptor(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
    ctx := context.WithValue(ss.Context(), myKey, myValue)
    return handler(srv, grpcx.WrapServerStream(ss, ctx))
}
```

### Metadata 操作

```go
// 读取入站 metadata
traceID := grpcx.GetMetadataValue(ctx, "x-trace-id")

// 设置出站 metadata（客户端调用前）
ctx = grpcx.AppendOutgoingMetadata(ctx, "x-trace-id", traceID)

// 代理场景：复制入站到出站
ctx = grpcx.CopyIncomingToOutgoing(ctx, "x-trace-id", "x-request-id")
```

### 错误处理

```go
// 创建错误
err := grpcx.NotFound("用户不存在")

// 检查错误类型
if grpcx.IsCode(err, codes.NotFound) {
    // 处理 404
}

// 提取信息
code := grpcx.Code(err)    // codes.NotFound
msg := grpcx.Message(err)  // "用户不存在"
```

### 健康检查

```go
if err := grpcx.HealthCheck(ctx, conn); err != nil {
    log.Fatalf("服务不可用: %v", err)
}

if err := grpcx.WaitForReady(ctx, conn, 5*time.Second); err != nil {
    log.Fatalf("连接超时: %v", err)
}
```
