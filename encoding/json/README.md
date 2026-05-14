# json

JSON 编解码器实现，基于标准库 `encoding/json`。

## 功能特性

- **标准 JSON 编解码**：基于 Go 标准库 `encoding/json` 实现
- **自动注册**：通过 `init()` 函数自动注册到 `encoding` 全局注册表
- **零配置**：导入包即可使用，无需手动初始化

## 安装

```bash
go get github.com/Tsukikage7/servex/v2/encoding
```

## API 参考

本包导出一个实现了 `encoding.Codec` 接口的编解码器，注册名称为 `"json"`。

| 方法 | 说明 |
| --- | --- |
| `Marshal(v any) ([]byte, error)` | 调用 `encoding/json.Marshal` |
| `Unmarshal(data []byte, v any) error` | 调用 `encoding/json.Unmarshal` |
| `Name() string` | 返回 `"json"` |

## 注意事项

- 本包仅需通过空白导入（`_ "github.com/Tsukikage7/servex/v2/encoding/json"`）即可完成注册，不需要直接调用包内任何函数。
- 编解码行为与标准库 `encoding/json` 完全一致，包括结构体标签（`json:"..."`）的处理。
- 如需在 Protobuf 消息中保留零值字段，请使用 `encoding/proto` 子包。
