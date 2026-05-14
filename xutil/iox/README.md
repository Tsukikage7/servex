# xutil/iox

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/xutil/iox"
```

## 简介

`xutil/iox` 提供 IO 工具函数集合，简化常见的读写、关闭和流处理操作。包括安全读取全部内容、按行读取、带限制的读取器、多 Closer 聚合，以及静默关闭和日志关闭等实用函数。

## 核心函数

| 函数 | 说明 |
|---|---|
| `ReadAll(r)` | 读取全部内容（同 io.ReadAll，附加错误处理） |
| `ReadString(r)` | 读取全部内容并返回字符串 |
| `ReadLines(r)` | 按行读取，返回字符串切片 |
| `Drain(r)` | 丢弃全部内容（用于释放连接） |
| `WriteString(w, s)` | 向 Writer 写入字符串 |
| `MultiCloser(closers...)` | 聚合多个 `io.Closer`，`Close()` 时依次关闭 |
| `CloseAndLog(c, logger, msg)` | 关闭并记录错误日志 |
| `LimitReadCloser(rc, n)` | 返回限制最大读取字节数的 ReadCloser |

## 示例

```go
package main

import (
    "fmt"
    "net/http"
    "strings"

    "github.com/Tsukikage7/servex/v2/xutil/iox"
)

func main() {
    // 读取字符串内容
    r := strings.NewReader("Hello, World!\nLine 2\nLine 3")
    lines, err := iox.ReadLines(r)
    if err != nil {
        panic(err)
    }
    fmt.Println("行数:", len(lines)) // 3
    for i, line := range lines {
        fmt.Printf("  第%d行: %s\n", i+1, line)
    }

    // 限制读取大小（防止超大请求体）
    resp, err := http.Get("https://example.com")
    if err != nil {
        return
    }
    limited := iox.LimitReadCloser(resp.Body, 1024*1024) // 最多读 1MB
    defer limited.Close()

    content, _ := iox.ReadString(limited)
    fmt.Printf("读取了 %d 字节\n", len(content))

    // 聚合关闭多个资源
    r1 := strings.NewReader("a")
    r2 := strings.NewReader("b")
    mc := iox.MultiCloser(
        iox.LimitReadCloser(r1, 10),
        iox.LimitReadCloser(r2, 10),
    )
    defer mc.Close() // 依次关闭所有资源
}
```
