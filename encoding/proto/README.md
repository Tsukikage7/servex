# proto

Protobuf JSON 编解码器实现，对 `proto.Message` 使用 `protojson` 序列化，非 `proto.Message` 类型回退到标准 JSON。

## 功能特性

- **Protobuf JSON 编解码**：对 `proto.Message` 类型使用 `protojson` 进行序列化和反序列化
- **零值保留**：序列化时保留零值字段（`EmitUnpopulated=true`），使用 proto 字段名（`UseProtoNames=true`）
- **自动回退**：非 `proto.Message` 类型自动回退到标准库 `encoding/json`
- **自动注册**：通过 `init()` 函数自动注册到 `encoding` 全局注册表

## 安装

```bash
go get github.com/Tsukikage7/servex/v2/encoding
```

## API 参考

本包导出一个实现了 `encoding.Codec` 接口的编解码器，注册名称为 `"proto"`。

| 方法 | 说明 |
| --- | --- |
| `Marshal(v any) ([]byte, error)` | proto.Message 使用 `pbjson.Marshal`，其他类型使用 `encoding/json.Marshal` |
| `Unmarshal(data []byte, v any) error` | proto.Message 使用 `pbjson.Unmarshal`，其他类型使用 `encoding/json.Unmarshal` |
| `Name() string` | 返回 `"proto"` |

### pbjson MarshalOptions

本编解码器通过 `pbjson` 包使用以下 `protojson.MarshalOptions`：

| 选项 | 值 | 说明 |
| --- | --- | --- |
| `EmitUnpopulated` | `true` | 序列化时包含零值字段 |
| `UseProtoNames` | `true` | 使用 proto 定义中的字段名而非 camelCase |

## 注意事项

- 本包仅需通过空白导入（`_ "github.com/Tsukikage7/servex/v2/encoding/proto"`）即可完成注册。
- 序列化输出为 JSON 格式（protojson），而非 Protobuf 二进制格式。
- 零值字段（如 `int32` 的 `0`、`bool` 的 `false`、`string` 的 `""`）会被保留在输出中，解决了标准 `encoding/json` 忽略零值的问题。
- 非 `proto.Message` 类型的回退行为与标准库 `encoding/json` 一致。
