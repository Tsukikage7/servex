package gemini

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"

	"github.com/Tsukikage7/servex/v2/llm"
)

// streamReader 实现 llm.StreamReader，解析 Gemini SSE 流.
type streamReader struct {
	scanner      *bufio.Scanner
	body         io.ReadCloser
	response     *llm.ChatResponse
	closed       bool
	fullContent  strings.Builder
	toolCalls    []llm.ToolCall
	finishReason string
	modelID      string
	usage        llm.Usage
}

// Recv 读取下一个流式片段.
func (r *streamReader) Recv() (llm.StreamChunk, error) {
	if r.closed {
		return llm.StreamChunk{}, llm.ErrStreamClosed
	}

	for r.scanner.Scan() {
		line := r.scanner.Text()
		if line == "" || line == "[" || line == "]" || line == "," {
			continue
		}

		// 移除前缀 "data: "（Gemini 使用 Server-Sent Events）
		if strings.HasPrefix(line, "data: ") {
			line = strings.TrimPrefix(line, "data: ")
		}

		// 移除 JSON 数组分隔符
		line = strings.TrimLeft(line, " ")
		if line == "" {
			continue
		}

		var chunk generateResponse
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			continue
		}

		return r.processChunk(chunk)
	}

	if err := r.scanner.Err(); err != nil {
		return llm.StreamChunk{}, err
	}
	r.buildResponse()
	return llm.StreamChunk{}, io.EOF
}

// processChunk 处理单个响应块.
func (r *streamReader) processChunk(chunk generateResponse) (llm.StreamChunk, error) {
	if r.modelID == "" && chunk.ModelVersion != "" {
		r.modelID = chunk.ModelVersion
	}
	if chunk.UsageMetadata != nil {
		r.usage = llm.Usage{
			PromptTokens:     chunk.UsageMetadata.PromptTokenCount,
			CompletionTokens: chunk.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      chunk.UsageMetadata.TotalTokenCount,
		}
	}

	if len(chunk.Candidates) == 0 {
		return llm.StreamChunk{}, nil
	}

	candidate := chunk.Candidates[0]
	if candidate.FinishReason != "" {
		r.finishReason = candidate.FinishReason
		r.buildResponse()
		return llm.StreamChunk{FinishReason: candidate.FinishReason}, io.EOF
	}

	result := llm.StreamChunk{}
	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			result.Delta += part.Text
			r.fullContent.WriteString(part.Text)
		}
		if part.FunctionCall != nil {
			args, _ := json.Marshal(part.FunctionCall.Args)
			call := llm.ToolCall{}
			call.Function.Name = part.FunctionCall.Name
			call.Function.Arguments = string(args)
			r.toolCalls = append(r.toolCalls, call)
			result.ToolCalls = append(result.ToolCalls, call)
		}
	}

	if result.Delta != "" || len(result.ToolCalls) > 0 {
		return result, nil
	}
	return llm.StreamChunk{}, nil
}

// buildResponse 构建最终完整响应.
func (r *streamReader) buildResponse() {
	if r.response != nil {
		return
	}
	msg := llm.AssistantMessage(r.fullContent.String())
	if len(r.toolCalls) > 0 {
		msg.ToolCalls = r.toolCalls
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
