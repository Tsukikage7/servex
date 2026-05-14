# xml

XML 编解码器实现，基于标准库 `encoding/xml`。

## 功能特性

- **标准 XML 编解码**：基于 Go 标准库 `encoding/xml` 实现
- **自动注册**：通过 `init()` 函数自动注册到 `encoding` 全局注册表
- **零配置**：导入包即可使用，无需手动初始化

## 安装

```bash
go get github.com/Tsukikage7/servex/v2/encoding
```

## API 参考

本包导出一个实现了 `encoding.Codec` 接口的编解码器，注册名称为 `"xml"`。

| 方法 | 说明 |
| --- | --- |
| `Marshal(v any) ([]byte, error)` | 调用 `encoding/xml.Marshal` |
| `Unmarshal(data []byte, v any) error` | 调用 `encoding/xml.Unmarshal` |
| `Name() string` | 返回 `"xml"` |

## 注意事项

- 本包仅需通过空白导入（`_ "github.com/Tsukikage7/servex/v2/encoding/xml"`）即可完成注册，不需要直接调用包内任何函数。
- 编解码行为与标准库 `encoding/xml` 完全一致，包括结构体标签（`xml:"..."`）的处理。
- HTTP 内容协商时，`Content-Type: application/xml` 和 `Content-Type: text/xml` 均会匹配到本编解码器。
