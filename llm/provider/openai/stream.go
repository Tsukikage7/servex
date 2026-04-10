package openai

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"

	"github.com/Tsukikage7/servex/llm"
)

// streamReader 实现 llm.StreamReader，解析 OpenAI SSE 流.
type streamReader struct {
	scanner  *bufio.Scanner
	body     io.ReadCloser
	response *llm.ChatResponse
	closed   bool

	// 累积完整响应数据
	fullContent  strings.Builder
	toolCalls    map[int]*llm.ToolCall // index -> ToolCall
	finishReason string
	modelID      string
	usage        llm.Usage
}

// chatCompletionChunk OpenAI 流式响应片段结构.
type chatCompletionChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Model   string `json:"model"`
	Choices []struct {
		Delta struct {
			Role      string          `json:"role"`
			Content   string          `json:"content"`
			ToolCalls []toolCallChunk `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
		Index        int     `json:"index"`
	} `json:"choices"`
	Usage *usageResponse `json:"usage"`
}

// toolCallChunk 流式工具调用片段.
type toolCallChunk struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// Recv 读取下一个流式片段.
func (r *streamReader) Recv() (llm.StreamChunk, error) {
	if r.closed {
		return llm.StreamChunk{}, llm.ErrStreamClosed
	}

	for r.scanner.Scan() {
		line := r.scanner.Text()

		// 跳过空行
		if line == "" {
			continue
		}

		// SSE 格式：以 "data: " 开头
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		// 流结束标志
		if data == "[DONE]" {
			r.buildResponse()
			return llm.StreamChunk{}, io.EOF
		}

		var chunk chatCompletionChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) == 0 {
			// 某些 Provider 最后一个 chunk 只包含 usage
			if chunk.Usage != nil {
				r.usage = llm.Usage{
					PromptTokens:     chunk.Usage.PromptTokens,
					CompletionTokens: chunk.Usage.CompletionTokens,
					TotalTokens:      chunk.Usage.TotalTokens,
				}
			}
			continue
		}

		choice := chunk.Choices[0]
		if chunk.Model != "" {
			r.modelID = chunk.Model
		}

		result := llm.StreamChunk{}

		// 文本 delta
		if choice.Delta.Content != "" {
			result.Delta = choice.Delta.Content
			r.fullContent.WriteString(choice.Delta.Content)
		}

		// 工具调用 delta
		if len(choice.Delta.ToolCalls) > 0 {
			if r.toolCalls == nil {
				r.toolCalls = make(map[int]*llm.ToolCall)
			}
			for _, tc := range choice.Delta.ToolCalls {
				if _, ok := r.toolCalls[tc.Index]; !ok {
					r.toolCalls[tc.Index] = &llm.ToolCall{}
				}
				existing := r.toolCalls[tc.Index]
				if tc.ID != "" {
					existing.ID = tc.ID
				}
				if tc.Function.Name != "" {
					existing.Function.Name = tc.Function.Name
				}
				existing.Function.Arguments += tc.Function.Arguments
			}
		}

		// 停止原因
		if choice.FinishReason != nil {
			result.FinishReason = *choice.FinishReason
			r.finishReason = result.FinishReason
		}

		return result, nil
	}

	if err := r.scanner.Err(); err != nil {
		return llm.StreamChunk{}, err
	}

	r.buildResponse()
	return llm.StreamChunk{}, io.EOF
}

// buildResponse 构建最终完整响应.
func (r *streamReader) buildResponse() {
	if r.response != nil {
		return
	}

	msg := llm.AssistantMessage(r.fullContent.String())

	// 组装工具调用
	if len(r.toolCalls) > 0 {
		calls := make([]llm.ToolCall, 0, len(r.toolCalls))
		for i := range len(r.toolCalls) {
			if tc, ok := r.toolCalls[i]; ok {
				calls = append(calls, *tc)
			}
		}
		msg.ToolCalls = calls
	}

	r.response = &llm.ChatResponse{
		Message:      msg,
		Usage:        r.usage,
		FinishReason: r.finishReason,
		ModelID:      r.modelID,
	}
}

// Response 获取累积的完整响应，流结束后可用.
func (r *streamReader) Response() *llm.ChatResponse {
	return r.response
}

// Close 关闭流.
func (r *streamReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	return r.body.Close()
}

// 编译期接口断言.
var _ llm.StreamReader = (*streamReader)(nil)
