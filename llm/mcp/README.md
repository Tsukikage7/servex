# llm/mcp

`github.com/Tsukikage7/servex/v2/llm/mcp` 提供 MCP 工具接入的最小边界：工具注册、最小权限策略、调用分发和 `llm.Tool` 转换。

这个包不实现 Agent loop、planner、长期 memory 或 MCP transport runtime。生产环境应在业务服务中显式连接 MCP server，并通过 `Policy` 控制模型可见的工具集合。

## 使用示例

```go
registry := mcp.NewRegistry(mcp.Policy{Allow: []string{"order.lookup"}})
_ = registry.Register(mcp.Tool{
    Name:        "order.lookup",
    Description: "查询订单",
    InputSchema: json.RawMessage(`{"type":"object"}`),
    Handler: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
        return args, nil
    },
})

tools := registry.LLMTools()
_ = tools
```
