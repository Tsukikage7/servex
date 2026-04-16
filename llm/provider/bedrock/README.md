# llm/provider/bedrock

AWS Bedrock（Converse API）适配器，实现 `llm.ChatModel` 接口。

支持所有通过 Bedrock Converse API 可用的模型：Anthropic Claude、Amazon Titan、Meta Llama、Mistral 等。

## 安装

```bash
go get github.com/aws/aws-sdk-go-v2/service/bedrockruntime
```

## 快速上手

```go
import (
    "context"

    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/bedrockruntime"

    "github.com/Tsukikage7/servex/v2/llm"
    "github.com/Tsukikage7/servex/v2/llm/provider/bedrock"
)

func main() {
    // 加载 AWS 默认凭证（环境变量 / ~/.aws/credentials / IAM Role 均可）
    cfg, err := config.LoadDefaultConfig(context.Background(),
        config.WithRegion("us-east-1"),
    )
    if err != nil {
        panic(err)
    }

    brClient := bedrockruntime.NewFromConfig(cfg)
    client := bedrock.New(brClient,
        bedrock.WithModel("anthropic.claude-3-5-sonnet-20241022-v2:0"),
    )

    resp, err := client.Generate(context.Background(), []llm.Message{
        llm.SystemMessage("你是一个专业助手"),
        llm.UserMessage("介绍一下 AWS Bedrock"),
    }, llm.WithMaxTokens(512))
    if err != nil {
        panic(err)
    }
    fmt.Println(resp.Message.Content)
}
```

## 流式生成

```go
reader, err := client.Stream(ctx, messages)
if err != nil {
    panic(err)
}
defer reader.Close()

for {
    chunk, err := reader.Recv()
    if errors.Is(err, io.EOF) {
        break
    }
    if err != nil {
        panic(err)
    }
    fmt.Print(chunk.Delta)
}
```

## 工具调用（Function Calling）

```go
weatherTool := llm.Tool{
    Function: llm.FunctionDef{
        Name:        "get_weather",
        Description: "获取指定城市的当前天气",
        Parameters:  json.RawMessage(`{
            "type": "object",
            "properties": {
                "location": {"type": "string", "description": "城市名称"}
            },
            "required": ["location"]
        }`),
    },
}

resp, err := client.Generate(ctx, messages, llm.WithTools(weatherTool))
```

## 凭证配置

支持所有标准 AWS 凭证来源：

| 方式 | 说明 |
|---|---|
| 环境变量 | `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` / `AWS_REGION` |
| 配置文件 | `~/.aws/credentials` + `~/.aws/config` |
| IAM Role | EC2/ECS/Lambda 实例角色，自动获取临时凭证 |
| `config.WithCredentialsProvider` | 自定义凭证 Provider |

## Bedrock Converse API 特性

- **system 消息**：通过 `ConverseInput.System` 字段传递，不在 `Messages` 列表中
- **角色**：只支持 `user` 和 `assistant`，`tool` 结果消息转换为 `user` 角色下的 `ToolResultBlock`
- **工具调用**：assistant 消息的 `ContentBlock` 中包含 `ToolUseBlock`，工具结果放在下一条 user 消息中

## 支持的模型 ID（示例）

| 模型 | ID |
|---|---|
| Claude 3.5 Sonnet v2 | `anthropic.claude-3-5-sonnet-20241022-v2:0` |
| Claude 3 Haiku | `anthropic.claude-3-haiku-20240307-v1:0` |
| Amazon Titan Text G1 | `amazon.titan-text-express-v1` |
| Meta Llama 3.1 70B | `meta.llama3-1-70b-instruct-v1:0` |
| Mistral Large | `mistral.mistral-large-2402-v1:0` |

完整列表参见 [Amazon Bedrock 模型 ID 文档](https://docs.aws.amazon.com/bedrock/latest/userguide/model-ids.html)。
