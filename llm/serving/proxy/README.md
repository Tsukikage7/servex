# llm/serving/proxy

`github.com/Tsukikage7/servex/v2/llm/serving/proxy` — OpenAI Chat Completions 兼容的最小代理网关，支持多 Provider 注册、按模型名称路由、API Key 鉴权、内容审核和计费。

## 核心类型

- `Proxy` — 代理网关主体
- `ProviderConfig` — Provider 配置，包含 Name、Models、Weight（负载均衡权重）、Priority（故障转移优先级）
- `New(providers, opts...)` — 创建 Proxy 实例
- `RegisterProvider(name, model, models, opts...)` — 注册 Provider 并绑定支持的模型名列表
- `Route(model)` — 根据模型名称选择对应 Provider
- `Handler()` — 返回 OpenAI 兼容的 HTTP Handler，注册以下路由：
  - `POST /v1/chat/completions` — 聊天补全（支持流式 SSE）
  - `GET /v1/models` — 模型列表
- `WithAPIKeyManager(mgr)` — 设置 API Key 鉴权
- `WithBilling(b)` — 设置计费引擎
- `WithModeration(mod)` — 设置内容审核器
- `WithLogger(log)` — 设置日志记录器

当前代理覆盖文本聊天的常用字段，不承诺完整 OpenAI API 兼容；工具流式增量、多模态、response_format 等高级字段需要按业务扩展。

## 使用示例

```go
import "github.com/Tsukikage7/servex/v2/llm/serving/proxy"

p := proxy.New(nil,
    proxy.WithAPIKeyManager(keyMgr),
    proxy.WithBilling(billingEngine),
    proxy.WithLogger(logger),
)

p.RegisterProvider("openai", openaiModel,
    []string{"gpt-4o", "gpt-4o-mini"},
    proxy.WithWeight(10),
)
p.RegisterProvider("claude", claudeModel,
    []string{"claude-3-5-sonnet"},
    proxy.WithWeight(5),
)

http.ListenAndServe(":8080", p.Handler())
```
