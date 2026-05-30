package anthropic

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"

	"github.com/Tsukikage7/servex/v2/llm"
)

// streamReader 实现 llm.StreamReader，解析 Anthropic SSE 流.
type streamReader struct {
	scanner      *bufio.Scanner
	body         io.ReadCloser
	response     *llm.ChatResponse
	closed       bool
	fullContent  strings.Builder
	toolCalls    map[int]*partialToolCall
	finishReason string
	modelID      string
	usage        llm.Usage
}

// partialToolCall 流式工具调用累积.
type partialToolCall struct {
	id    string
	name  string
	input strings.Builder
}

// sseEvent Anthropic SSE 事件.
type sseEvent struct {
	Type  string          `json:"type"`
	Index int             `json:"index"`
	Delta json.RawMessage `json:"delta"`
	Usage *struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Message *struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Usage *struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	} `json:"message"`
	ContentBlock *struct {
		Type  string `json:"type"`
		ID    string `json:"id"`
		Name  string `json:"name"`
		Input string `json:"input"`
	} `json:"content_block"`
}

// deltaPayload delta 内容.
type deltaPayload struct {
	Type        string `json:"type"`
	Text        string `json:"text"`
	PartialJSON string `json:"partial_json"`
	StopReason  string `json:"stop_reason"`
}

// Recv 读取下一个流式片段.
func (r *streamReader) Recv() (llm.StreamChunk, error) {
	if r.closed {
		return llm.StreamChunk{}, llm.ErrStreamClosed
	}

	for r.scanner.Scan() {
		line := r.scanner.Text()
		if line == "" {
			continue
		}

		// event: 行跳过，仅处理 data:
		if strings.HasPrefix(line, "event: ") {
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		var event sseEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "message_start":
			if event.Message != nil {
				r.modelID = event.Message.Model
				if event.Message.Usage != nil {
					r.usage.PromptTokens = event.Message.Usage.InputTokens
				}
			}
		case "content_block_start":
			if event.ContentBlock != nil {
				if event.ContentBlock.Type == "tool_use" {
					if r.toolCalls == nil {
						r.toolCalls = make(map[int]*partialToolCall)
					}
					r.toolCalls[event.Index] = &partialToolCall{
						id:   event.ContentBlock.ID,
						name: event.ContentBlock.Name,
					}
				}
			}
		case "content_block_delta":
			if len(event.Delta) == 0 {
				continue
			}
			var delta deltaPayload
			if err := json.Unmarshal(event.Delta, &delta); err != nil {
				continue
			}
			switch delta.Type {
			case "text_delta":
				r.fullContent.WriteString(delta.Text)
				return llm.StreamChunk{Delta: delta.Text}, nil
			case "input_json_delta":
				if tc, ok := r.toolCalls[event.Index]; ok {
					tc.input.WriteString(delta.PartialJSON)
				}
			}
		case "content_block_stop":
			// 工具调用完成，生成 chunk
			if r.toolCalls != nil {
				if tc, ok := r.toolCalls[event.Index]; ok {
					call := llm.ToolCall{ID: tc.id}
					call.Function.Name = tc.name
					call.Function.Arguments = tc.input.String()
					return llm.StreamChunk{ToolCalls: []llm.ToolCall{call}}, nil
				}
			}
		case "message_delta":
			if len(event.Delta) > 0 {
				var delta deltaPayload
				if err := json.Unmarshal(event.Delta, &delta); err == nil {
					r.finishReason = delta.StopReason
				}
			}
			if event.Usage != nil {
				r.usage.CompletionTokens = event.Usage.OutputTokens
				r.usage.TotalTokens = r.usage.PromptTokens + r.usage.CompletionTokens
			}
		case "message_stop":
			r.buildResponse()
			return llm.StreamChunk{}, io.EOF
		}
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
	if len(r.toolCalls) > 0 {
		calls := make([]llm.ToolCall, 0, len(r.toolCalls))
		for i := range len(r.toolCalls) {
			if tc, ok := r.toolCalls[i]; ok {
				call := llm.ToolCall{ID: tc.id}
				call.Function.Name = tc.name
				call.Function.Arguments = tc.input.String()
				calls = append(calls, call)
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

// Response 获取完整响应.
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
