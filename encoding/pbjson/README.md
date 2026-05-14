# pbjson

Protobuf JSON 序列化工具，解决零值忽略问题。

## 使用

### JSON 序列化

```go
import "github.com/Tsukikage7/servex/v2/encoding/pbjson"

// 序列化（包含零值字段）
data, err := pbjson.Marshal(protoMsg)

// 反序列化
err := pbjson.Unmarshal(data, protoMsg)
```

### HTTP 响应编码

```go
import (
    "github.com/Tsukikage7/servex/v2/encoding/pbjson"
    "github.com/Tsukikage7/servex/v2/transport/httpserver"
)

handler := httpserver.NewEndpointHandler(
    endpoint,
    decodeRequest,
    pbjson.EncodeResponse,  // 零值字段会输出
)
```

### HTTP 请求解码

```go
func decodeCreateOrderRequest(_ context.Context, r *http.Request) (any, error) {
    req := &pb.CreateOrderRequest{}
    if err := pbjson.DecodeRequest(r, req); err != nil {
        return nil, err
    }
    return req, nil
}
```

## 效果对比

```json
// 标准 json.Marshal（忽略零值）
{
    "id": "order-1"
}

// pbjson.Marshal（包含零值）
{
    "id": "order-1",
    "code": 0,
    "enabled": false,
    "name": ""
}
```
