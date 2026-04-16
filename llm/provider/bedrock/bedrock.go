// Package bedrock 提供 AWS Bedrock（Converse API）适配器.
// 支持所有 Bedrock 上的模型：Claude、Titan、Llama、Mistral 等.
package bedrock

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"

	"github.com/Tsukikage7/servex/v2/llm"
)

// Option 客户端配置选项.
type Option func(*Client)

// WithModel 设置默认模型 ID.
func WithModel(model string) Option { return func(c *Client) { c.model = model } }

// Client AWS Bedrock Converse API 客户端.
type Client struct {
	client *bedrockruntime.Client
	model  string // 如 "anthropic.claude-3-5-sonnet-20241022-v2:0"
}

// 编译期接口断言.
var _ llm.ChatModel = (*Client)(nil)

// New 创建 Bedrock 客户端.
func New(brClient *bedrockruntime.Client, opts ...Option) *Client {
	c := &Client{
		client: brClient,
		model:  "anthropic.claude-3-5-sonnet-20241022-v2:0",
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Generate 非流式生成.
func (c *Client) Generate(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
	co := llm.ApplyOptions(opts)
	model := c.model
	if co.Model != "" {
		model = co.Model
	}

	input := &bedrockruntime.ConverseInput{
		ModelId: &model,
	}

	// 分离 system 消息与对话消息
	sysBlocks, convMsgs := splitMessages(messages)
	input.Messages = convMsgs
	if len(sysBlocks) > 0 {
		input.System = sysBlocks
	}

	// 推理参数
	if co.Temperature != nil || co.MaxTokens != nil || co.TopP != nil {
		input.InferenceConfig = &types.InferenceConfiguration{}
		if co.Temperature != nil {
			temp := float32(*co.Temperature)
			input.InferenceConfig.Temperature = &temp
		}
		if co.MaxTokens != nil {
			mt := int32(*co.MaxTokens)
			input.InferenceConfig.MaxTokens = &mt
		}
		if co.TopP != nil {
			tp := float32(*co.TopP)
			input.InferenceConfig.TopP = &tp
		}
	}

	// 工具
	if len(co.Tools) > 0 {
		input.ToolConfig = convertToolConfig(co.Tools)
	}

	resp, err := c.client.Converse(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("bedrock: converse: %w", err)
	}

	return convertResponse(resp), nil
}

// Stream 流式生成.
func (c *Client) Stream(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (llm.StreamReader, error) {
	co := llm.ApplyOptions(opts)
	model := c.model
	if co.Model != "" {
		model = co.Model
	}

	input := &bedrockruntime.ConverseStreamInput{
		ModelId: &model,
	}

	// 分离 system 消息与对话消息
	sysBlocks, convMsgs := splitMessages(messages)
	input.Messages = convMsgs
	if len(sysBlocks) > 0 {
		input.System = sysBlocks
	}

	// 推理参数
	if co.Temperature != nil || co.MaxTokens != nil || co.TopP != nil {
		input.InferenceConfig = &types.InferenceConfiguration{}
		if co.Temperature != nil {
			temp := float32(*co.Temperature)
			input.InferenceConfig.Temperature = &temp
		}
		if co.MaxTokens != nil {
			mt := int32(*co.MaxTokens)
			input.InferenceConfig.MaxTokens = &mt
		}
		if co.TopP != nil {
			tp := float32(*co.TopP)
			input.InferenceConfig.TopP = &tp
		}
	}

	// 工具
	if len(co.Tools) > 0 {
		input.ToolConfig = convertToolConfig(co.Tools)
	}

	out, err := c.client.ConverseStream(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("bedrock: converse stream: %w", err)
	}

	return &bedrockStreamReader{
		ctx:       ctx,
		stream:    out.GetStream(),
		toolCalls: make(map[int]*llm.ToolCall),
	}, nil
}

// ─── Stream Reader ──────────────────────────────────────────────────────────

type bedrockStreamReader struct {
	ctx          context.Context
	stream       *bedrockruntime.ConverseStreamEventStream
	content      string
	toolCalls    map[int]*llm.ToolCall // index → ToolCall（流式累积）
	usage        llm.Usage
	finishReason string
	modelID      string
	done         bool
}

// Recv 读取下一个流式片段.
func (r *bedrockStreamReader) Recv() (llm.StreamChunk, error) {
	if r.done {
		return llm.StreamChunk{}, io.EOF
	}

	for {
		select {
		case <-r.ctx.Done():
			r.done = true
			return llm.StreamChunk{}, r.ctx.Err()
		case event, ok := <-r.stream.Events():
			if !ok {
				// channel 已关闭，流结束
				if err := r.stream.Err(); err != nil {
					return llm.StreamChunk{}, fmt.Errorf("bedrock: stream error: %w", err)
				}
				r.done = true
				// 若有工具调用，最后一个 chunk 携带完整的 ToolCalls
				var finalToolCalls []llm.ToolCall
				for i := 0; i < len(r.toolCalls); i++ {
					if tc, ok := r.toolCalls[i]; ok {
						finalToolCalls = append(finalToolCalls, *tc)
					}
				}
				return llm.StreamChunk{
					ToolCalls:    finalToolCalls,
					FinishReason: r.finishReason,
				}, io.EOF
			}

			switch v := event.(type) {

			case *types.ConverseStreamOutputMemberContentBlockStart:
				// 流式工具调用开始：记录 toolUse ID 和 name
				if start := v.Value.Start; start != nil {
					if tu, ok := start.(*types.ContentBlockStartMemberToolUse); ok {
						idx := int(*v.Value.ContentBlockIndex)
						tc := &llm.ToolCall{}
						if tu.Value.ToolUseId != nil {
							tc.ID = *tu.Value.ToolUseId
						}
						if tu.Value.Name != nil {
							tc.Function.Name = *tu.Value.Name
						}
						r.toolCalls[idx] = tc
					}
				}

			case *types.ConverseStreamOutputMemberContentBlockDelta:
				delta := v.Value.Delta
				switch d := delta.(type) {
				case *types.ContentBlockDeltaMemberText:
					r.content += d.Value
					return llm.StreamChunk{Delta: d.Value}, nil

				case *types.ContentBlockDeltaMemberToolUse:
					// 累积 tool use arguments
					idx := int(*v.Value.ContentBlockIndex)
					if tc, ok := r.toolCalls[idx]; ok {
						tc.Function.Arguments += *d.Value.Input
					}
				}

			case *types.ConverseStreamOutputMemberMessageStop:
				r.finishReason = string(v.Value.StopReason)

			case *types.ConverseStreamOutputMemberMetadata:
				if v.Value.Usage != nil {
					u := v.Value.Usage
					if u.InputTokens != nil {
						r.usage.PromptTokens = int(*u.InputTokens)
					}
					if u.OutputTokens != nil {
						r.usage.CompletionTokens = int(*u.OutputTokens)
					}
					if u.TotalTokens != nil {
						r.usage.TotalTokens = int(*u.TotalTokens)
					}
				}
			}
		}
	}
}

// Response 流结束后返回完整响应.
func (r *bedrockStreamReader) Response() *llm.ChatResponse {
	if !r.done {
		return nil
	}
	msg := llm.Message{
		Role:    llm.RoleAssistant,
		Content: r.content,
	}
	if len(r.toolCalls) > 0 {
		for i := 0; i < len(r.toolCalls); i++ {
			if tc, ok := r.toolCalls[i]; ok {
				msg.ToolCalls = append(msg.ToolCalls, *tc)
			}
		}
	}
	return &llm.ChatResponse{
		Message:      msg,
		Usage:        r.usage,
		FinishReason: r.finishReason,
		ModelID:      r.modelID,
	}
}

// Close 关闭事件流.
func (r *bedrockStreamReader) Close() error {
	return r.stream.Close()
}
