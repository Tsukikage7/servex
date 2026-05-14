# encoding

编解码器接口与全局注册表，支持 HTTP 内容协商自动选择编解码器。

## 功能特性

- **统一编解码接口**：定义 `Codec` 接口，规范 Marshal/Unmarshal/Name 方法
- **全局注册表**：线程安全的编解码器注册与查找
- **HTTP 内容协商**：根据 Accept/Content-Type 请求头自动选择编解码器
- **MIME 子类型解析**：支持 `application/json`、`application/xml`、`application/x-protobuf`、`vnd.api+json` 等格式
- **自动回退**：未匹配时回退到 JSON 编解码器
- **子包自注册**：`json`、`xml`、`proto` 子包通过 `init()` 自动注册

## 安装

```bash
go get github.com/Tsukikage7/servex/v2/encoding
```

## API 参考

### Codec 接口

```go
type Codec interface {
    Marshal(v any) ([]byte, error)     // 编码
    Unmarshal(data []byte, v any) error // 解码
    Name() string                      // 编解码器名称（如 "json", "xml", "proto"）
}
```

### 注册表函数

| 函数 | 说明 |
| --- | --- |
| `RegisterCodec(codec Codec)` | 注册编解码器到全局注册表，同名覆盖 |
| `GetCodec(name string) Codec` | 按名称获取编解码器，未找到返回 nil |
| `CodecForRequest(r *http.Request, headerName string) Codec` | 根据 HTTP 请求头选择编解码器 |

### 错误

| 变量 | 说明 |
| --- | --- |
| `ErrCodecNotFound` | 未找到匹配的编解码器 |

### MIME 类型映射

`CodecForRequest` 从 HTTP 头的 MIME 类型中提取子类型来匹配编解码器：

| MIME 类型 | 解析结果 |
| --- | --- |
| `application/json` | `json` |
| `application/xml` | `xml` |
| `text/xml` | `xml` |
| `application/x-protobuf` | `proto` |
| `application/json; charset=utf-8` | `json` |
| `application/vnd.api+json` | `json` |

## 子包

| 子包 | 说明 |
| --- | --- |
| `encoding/json` | JSON 编解码器，基于标准库 `encoding/json` |
| `encoding/xml` | XML 编解码器，基于标准库 `encoding/xml` |
| `encoding/proto` | Protobuf JSON 编解码器，基于 `protojson` |

导入子包即可自动注册，无需手动调用 `RegisterCodec`：

```go
import (
    _ "github.com/Tsukikage7/servex/v2/encoding/json"
    _ "github.com/Tsukikage7/servex/v2/encoding/xml"
    _ "github.com/Tsukikage7/servex/v2/encoding/proto"
)
```

## 注意事项

- `CodecForRequest` 在未匹配到编解码器时默认回退到 JSON，因此必须确保 JSON 编解码器已注册。
- `RegisterCodec` 对同名编解码器采用覆盖策略，后注册的会替换先注册的。
- 注册表操作是线程安全的，使用 `sync.RWMutex` 保护。
- 自定义编解码器只需实现 `Codec` 接口并调用 `RegisterCodec` 即可集成。
