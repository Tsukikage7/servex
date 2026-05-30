# ai-support

`ai-support` 展示如何把一个 AI 客服应用接入 Go 微服务体系。示例保持最小实现：HTTP API、LLM provider 初始化、业务 Agent 边界和本地工具注册，并复用 `transport/response` 的统一响应体与 i18n 错误输出。

这个示例不在业务样例里复刻 Agent runtime、RAG framework 或长期 memory。复杂编排通过 `llm/adapter/eino`、`llm/adapter/adk` 或业务层 runtime 接入；servex 负责模型访问、统一响应、i18n、微服务治理和服务边界。

## 运行

```bash
go mod tidy
OPENAI_API_KEY=sk-... just dev
```

请求：

```bash
curl -X POST http://localhost:8080/chat \
  -H 'content-type: application/json' \
  -d '{"message":"订单 VOYRA-1001 现在是什么状态？"}'
```

## 结构

```text
cmd/server/main.go       # HTTP 服务入口
internal/llm/model.go    # Provider、middleware 初始化
internal/agent/agent.go  # AI 客服业务边界
internal/tools/tools.go  # 业务工具入口
internal/http/chat.go    # /chat handler
configs/config.yaml      # 示例配置
```
