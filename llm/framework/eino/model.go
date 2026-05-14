package eino

import (
	"context"
	"errors"
	"io"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/Tsukikage7/servex/v2/llm"
)

// ChatModel 将 Eino BaseChatModel 适配为 servex llm.ChatModel.
type ChatModel struct {
	model model.BaseChatModel
}

// NewChatModel 创建 Eino 到 servex 的 ChatModel 适配器.
func NewChatModel(m model.BaseChatModel) (*ChatModel, error) {
	if m == nil {
		return nil, ErrNilModel
	}
	return &ChatModel{model: m}, nil
}

// Base 返回底层 Eino 模型.
func (m *ChatModel) Base() model.BaseChatModel {
	return m.model
}

// Generate 执行非流式模型调用.
func (m *ChatModel) Generate(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
	input, err := ToEinoMessages(messages)
	if err != nil {
		return nil, err
	}
	einoOpts, err := toEinoOptions(opts)
	if err != nil {
		return nil, err
	}
	resp, err := m.model.Generate(ctx, input, einoOpts...)
	if err != nil {
		return nil, err
	}
	return fromEinoResponse(resp), nil
}

// Stream 执行流式模型调用.
func (m *ChatModel) Stream(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (llm.StreamReader, error) {
	input, err := ToEinoMessages(messages)
	if err != nil {
		return nil, err
	}
	einoOpts, err := toEinoOptions(opts)
	if err != nil {
		return nil, err
	}
	reader, err := m.model.Stream(ctx, input, einoOpts...)
	if err != nil {
		return nil, err
	}
	return &streamReader{inner: reader}, nil
}

var _ llm.ChatModel = (*ChatModel)(nil)

func fromEinoResponse(msg *schema.Message) *llm.ChatResponse {
	resp := &llm.ChatResponse{Message: FromEinoMessage(msg)}
	if msg != nil && msg.ResponseMeta != nil {
		resp.FinishReason = msg.ResponseMeta.FinishReason
		resp.Usage = fromEinoUsage(msg.ResponseMeta.Usage)
	}
	return resp
}

func toEinoOptions(opts []llm.CallOption) ([]model.Option, error) {
	applied := llm.ApplyOptions(opts)
	out := make([]model.Option, 0, 6)
	if applied.Model != "" {
		out = append(out, model.WithModel(applied.Model))
	}
	if applied.Temperature != nil {
		out = append(out, model.WithTemperature(float32(*applied.Temperature)))
	}
	if applied.MaxTokens != nil {
		out = append(out, model.WithMaxTokens(*applied.MaxTokens))
	}
	if applied.TopP != nil {
		out = append(out, model.WithTopP(float32(*applied.TopP)))
	}
	if len(applied.Stop) > 0 {
		out = append(out, model.WithStop(applied.Stop))
	}
	if len(applied.Tools) > 0 {
		tools, err := ToEinoTools(applied.Tools)
		if err != nil {
			return nil, err
		}
		out = append(out, model.WithTools(tools))
	}
	if applied.ToolChoice != nil {
		switch applied.ToolChoice.Type {
		case "none":
			out = append(out, model.WithToolChoice(schema.ToolChoiceForbidden))
		case "required":
			out = append(out, model.WithToolChoice(schema.ToolChoiceForced))
		case "auto":
			out = append(out, model.WithToolChoice(schema.ToolChoiceAllowed))
		}
	}
	return out, nil
}

// AsChatModel 将 servex ChatModel 适配为 Eino BaseChatModel.
func AsChatModel(chatModel llm.ChatModel) (model.BaseChatModel, error) {
	if chatModel == nil {
		return nil, ErrNilModel
	}
	return &servexChatModel{model: chatModel}, nil
}

type servexChatModel struct {
	model llm.ChatModel
}

func (m *servexChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	messages := make([]llm.Message, 0, len(input))
	for _, msg := range input {
		messages = append(messages, FromEinoMessage(msg))
	}
	resp, err := m.model.Generate(ctx, messages, toServexOptions(opts)...)
	if err != nil {
		return nil, err
	}
	return toEinoResponse(resp)
}

func (m *servexChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	messages := make([]llm.Message, 0, len(input))
	for _, msg := range input {
		messages = append(messages, FromEinoMessage(msg))
	}
	reader, err := m.model.Stream(ctx, messages, toServexOptions(opts)...)
	if err != nil {
		return nil, err
	}
	out, writer := schema.Pipe[*schema.Message](0)
	go func() {
		defer writer.Close()
		defer reader.Close()
		for {
			chunk, err := reader.Recv()
			if errors.Is(err, io.EOF) {
				if resp := reader.Response(); resp != nil {
					msg, err := toEinoResponse(resp)
					if writer.Send(msg, err) {
						return
					}
				}
				return
			}
			if err != nil {
				_ = writer.Send(nil, err)
				return
			}
			msg, err := toEinoResponse(&llm.ChatResponse{
				Message:      llm.AssistantMessage(chunk.Delta),
				FinishReason: chunk.FinishReason,
			})
			if writer.Send(msg, err) {
				return
			}
		}
	}()
	return out, nil
}

func toServexOptions(opts []model.Option) []llm.CallOption {
	common := model.GetCommonOptions(nil, opts...)
	out := make([]llm.CallOption, 0, 5)
	if common.Model != nil {
		out = append(out, llm.WithModel(*common.Model))
	}
	if common.Temperature != nil {
		out = append(out, llm.WithTemperature(float64(*common.Temperature)))
	}
	if common.MaxTokens != nil {
		out = append(out, llm.WithMaxTokens(*common.MaxTokens))
	}
	if common.TopP != nil {
		out = append(out, llm.WithTopP(float64(*common.TopP)))
	}
	if len(common.Stop) > 0 {
		out = append(out, llm.WithStop(common.Stop...))
	}
	return out
}

func toEinoResponse(resp *llm.ChatResponse) (*schema.Message, error) {
	if resp == nil {
		return nil, nil
	}
	msg, err := ToEinoMessage(resp.Message)
	if err != nil {
		return nil, err
	}
	msg.ResponseMeta = &schema.ResponseMeta{
		FinishReason: resp.FinishReason,
		Usage: &schema.TokenUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}
	return msg, nil
}

type streamReader struct {
	inner *schema.StreamReader[*schema.Message]
	resp  *llm.ChatResponse
}

func (r *streamReader) Recv() (llm.StreamChunk, error) {
	msg, err := r.inner.Recv()
	if errors.Is(err, io.EOF) {
		return llm.StreamChunk{}, io.EOF
	}
	if err != nil {
		return llm.StreamChunk{}, err
	}
	resp := fromEinoResponse(msg)
	r.resp = resp
	return llm.StreamChunk{
		Delta:        resp.Message.Content,
		ToolCalls:    resp.Message.ToolCalls,
		FinishReason: resp.FinishReason,
	}, nil
}

func (r *streamReader) Response() *llm.ChatResponse {
	return r.resp
}

func (r *streamReader) Close() error {
	r.inner.Close()
	return nil
}
