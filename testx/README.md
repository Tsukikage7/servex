# testx

`github.com/Tsukikage7/servex/v2/testx` — 测试辅助工具集，提供 Mock 日志、HTTP 测试服务器及 fixture 文件管理。容器和 gRPC 测试辅助分别在 `testx/container`、`testx/grpcx` 子包中提供。

## 核心类型

- `NopLogger()` — 返回空操作日志记录器，实现 `logger.Logger` 但丢弃所有输出（适合单元测试）
- `TestLogger(t)` — 返回将日志输出到 `testing.T` 的日志记录器，测试失败时日志可见
- `HTTPTestServer` — 封装 `httptest.Server`，额外提供 `Get(path)`、`PostJSON(path, body)` 快捷方法
- `NewHTTPTestServer(handler, middlewares...)` — 创建 HTTP 测试服务器，支持中间件链
- `LoadJSON[T](t, path)` — 从文件加载 JSON 并反序列化为指定类型
- `LoadYAML[T](t, path)` — 从文件加载 YAML 并反序列化为指定类型
- `Golden(t, name, actual)` — 对比 actual 与 golden 文件，`-update` 标志可更新 golden 文件
- `GoldenJSON(t, name, actual)` — 将 actual 序列化为格式化 JSON 后与 golden 文件对比

## 使用示例

```go
import "github.com/Tsukikage7/servex/v2/testx"

func TestHTTPHandler(t *testing.T) {
    srv := testx.NewHTTPTestServer(myHandler)
    defer srv.Close()

    resp := srv.PostJSON("/api/hello", map[string]string{"name": "world"})
    // 断言 resp ...
}

// Golden 文件测试
func TestOutput(t *testing.T) {
    output := generateOutput()
    testx.GoldenJSON(t, "my_output", output)
}
```
