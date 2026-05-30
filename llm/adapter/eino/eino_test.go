package eino

import (
	"context"
	"errors"
	"io"
	"math"
	"testing"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/Tsukikage7/servex/v2/llm"
)

type stubEinoModel struct {
	input []*schema.Message
	opts  []model.Option
	resp  *schema.Message
	calls int
}

func (m *stubEinoModel) Generate(_ context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	m.input = input
	m.opts = opts
	m.calls++
	return m.resp, nil
}

func (m *stubEinoModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return schema.StreamReaderFromArray([]*schema.Message{m.resp}), nil
}

func TestMessageConversionPreservesRolesContentAndToolCalls(t *testing.T) {
	t.Parallel()

	msgs := []llm.Message{
		llm.SystemMessage("system"),
		llm.UserMessage("user"),
		{
			Role:    llm.RoleAssistant,
			Content: "assistant",
			ToolCalls: []llm.ToolCall{{
				ID: "call_1",
				Function: struct {
					Name      string
					Arguments string
				}{Name: "search", Arguments: `{"q":"go"}`},
			}},
		},
		llm.ToolResultMessage("call_1", `{"ok":true}`),
	}

	einoMsgs, err := ToEinoMessages(msgs)
	if err != nil {
		t.Fatalf("ToEinoMessages failed: %v", err)
	}
	if got := len(einoMsgs); got != len(msgs) {
		t.Fatalf("expected %d messages, got %d", len(msgs), got)
	}
	if einoMsgs[0].Role != schema.System || einoMsgs[0].Content != "system" {
		t.Fatalf("system message mismatch: %+v", einoMsgs[0])
	}
	if einoMsgs[2].Role != schema.Assistant || len(einoMsgs[2].ToolCalls) != 1 {
		t.Fatalf("assistant tool calls mismatch: %+v", einoMsgs[2])
	}
	if einoMsgs[2].ToolCalls[0].Function.Name != "search" {
		t.Fatalf("tool call name mismatch: %+v", einoMsgs[2].ToolCalls[0])
	}
	if einoMsgs[3].Role != schema.Tool || einoMsgs[3].ToolCallID != "call_1" {
		t.Fatalf("tool result mismatch: %+v", einoMsgs[3])
	}

	roundTrip := FromEinoMessage(einoMsgs[2])
	if roundTrip.Role != llm.RoleAssistant || len(roundTrip.ToolCalls) != 1 {
		t.Fatalf("round trip mismatch: %+v", roundTrip)
	}
}

func TestToolConversionPreservesJSONSchema(t *testing.T) {
	t.Parallel()

	tools, err := ToEinoTools([]llm.Tool{{
		Function: llm.FunctionDef{
			Name:        "search",
			Description: "搜索资料",
			Parameters:  []byte(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`),
		},
	}})
	if err != nil {
		t.Fatalf("ToEinoTools failed: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected one tool, got %d", len(tools))
	}
	if tools[0].Name != "search" || tools[0].Desc != "搜索资料" {
		t.Fatalf("tool metadata mismatch: %+v", tools[0])
	}
	if tools[0].ParamsOneOf == nil {
		t.Fatal("expected json schema params")
	}
}

func TestChatModelGenerateReturnsInvalidToolSchemaError(t *testing.T) {
	t.Parallel()

	base := &stubEinoModel{resp: schema.AssistantMessage("ok", nil)}
	model, err := NewChatModel(base)
	if err != nil {
		t.Fatalf("NewChatModel failed: %v", err)
	}

	_, err = model.Generate(t.Context(), []llm.Message{llm.UserMessage("hello")}, llm.WithTools(llm.Tool{
		Function: llm.FunctionDef{
			Name:       "broken",
			Parameters: []byte(`{"type":`),
		},
	}))
	if err == nil {
		t.Fatal("expected invalid tool schema error")
	}
	if base.calls != 0 {
		t.Fatalf("underlying model should not be called, got %d calls", base.calls)
	}
}

func TestChatModelGenerateDelegatesToEinoModel(t *testing.T) {
	t.Parallel()

	base := &stubEinoModel{resp: &schema.Message{
		Role:    schema.Assistant,
		Content: "ok",
		ResponseMeta: &schema.ResponseMeta{
			FinishReason: "stop",
			Usage:        &schema.TokenUsage{PromptTokens: 2, CompletionTokens: 3, TotalTokens: 5},
		},
	}}
	model, err := NewChatModel(base)
	if err != nil {
		t.Fatalf("NewChatModel failed: %v", err)
	}

	resp, err := model.Generate(t.Context(), []llm.Message{llm.UserMessage("hello")}, llm.WithModel("test-model"))
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if resp.Message.Content != "ok" || resp.FinishReason != "stop" {
		t.Fatalf("response mismatch: %+v", resp)
	}
	if resp.Usage.TotalTokens != 5 {
		t.Fatalf("usage mismatch: %+v", resp.Usage)
	}
	if len(base.input) != 1 || base.input[0].Content != "hello" {
		t.Fatalf("input not delegated: %+v", base.input)
	}
}

func TestChatModelStreamReturnsServexReader(t *testing.T) {
	t.Parallel()

	base := &stubEinoModel{resp: schema.AssistantMessage("chunk", nil)}
	model, err := NewChatModel(base)
	if err != nil {
		t.Fatalf("NewChatModel failed: %v", err)
	}

	reader, err := model.Stream(t.Context(), []llm.Message{llm.UserMessage("hello")})
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}
	defer reader.Close()

	chunk, err := reader.Recv()
	if err != nil {
		t.Fatalf("Recv failed: %v", err)
	}
	if chunk.Delta != "chunk" {
		t.Fatalf("chunk mismatch: %+v", chunk)
	}
	if _, err := reader.Recv(); err != io.EOF {
		t.Fatalf("expected EOF, got %v", err)
	}
}

type stubServexChatModel struct {
	messages []llm.Message
	opts     llm.CallOptions
	resp     *llm.ChatResponse
}

func (m *stubServexChatModel) Generate(_ context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
	m.messages = messages
	m.opts = llm.ApplyOptions(opts)
	return m.resp, nil
}

func (m *stubServexChatModel) Stream(context.Context, []llm.Message, ...llm.CallOption) (llm.StreamReader, error) {
	panic("not used")
}

func TestAsChatModelAdaptsServexModelToEino(t *testing.T) {
	t.Parallel()

	base := &stubServexChatModel{resp: &llm.ChatResponse{
		Message:      llm.AssistantMessage("ok"),
		FinishReason: "stop",
		Usage:        llm.Usage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3},
	}}
	chatModel, err := AsChatModel(base)
	if err != nil {
		t.Fatalf("AsChatModel failed: %v", err)
	}

	temp := float32(0.2)
	resp, err := chatModel.Generate(t.Context(), []*schema.Message{schema.UserMessage("hello")}, model.WithModel("m1"), model.WithTemperature(temp))
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if resp.Content != "ok" || resp.ResponseMeta == nil || resp.ResponseMeta.Usage.TotalTokens != 3 {
		t.Fatalf("response mismatch: %+v", resp)
	}
	if len(base.messages) != 1 || base.messages[0].Content != "hello" {
		t.Fatalf("servex input mismatch: %+v", base.messages)
	}
	if base.opts.Model != "m1" || base.opts.Temperature == nil || math.Abs(*base.opts.Temperature-0.2) > 0.000001 {
		t.Fatalf("servex options mismatch: %+v", base.opts)
	}
}

func TestAsChatModelReturnsConversionError(t *testing.T) {
	t.Parallel()

	base := &stubServexChatModel{resp: &llm.ChatResponse{
		Message: llm.Message{Role: llm.Role("unsupported"), Content: "bad"},
	}}
	chatModel, err := AsChatModel(base)
	if err != nil {
		t.Fatalf("AsChatModel failed: %v", err)
	}

	_, err = chatModel.Generate(t.Context(), []*schema.Message{schema.UserMessage("hello")})
	if !errors.Is(err, ErrUnsupportedMessage) {
		t.Fatalf("期望 ErrUnsupportedMessage, 得到 %v", err)
	}
}

type stubEinoEmbedder struct {
	texts []string
	opts  []embedding.Option
}

func (e *stubEinoEmbedder) EmbedStrings(_ context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	e.texts = texts
	e.opts = opts
	return [][]float64{{1.5, 2.5}}, nil
}

func TestEmbeddingModelAdaptsEinoEmbedder(t *testing.T) {
	t.Parallel()

	base := &stubEinoEmbedder{}
	model, err := NewEmbeddingModel(base)
	if err != nil {
		t.Fatalf("NewEmbeddingModel failed: %v", err)
	}
	resp, err := model.EmbedTexts(t.Context(), []string{"hello"}, llm.WithModel("emb"))
	if err != nil {
		t.Fatalf("EmbedTexts failed: %v", err)
	}
	if len(resp.Embeddings) != 1 || len(resp.Embeddings[0]) != 2 || resp.Embeddings[0][0] != 1.5 {
		t.Fatalf("embedding mismatch: %+v", resp.Embeddings)
	}
	if len(base.texts) != 1 || base.texts[0] != "hello" {
		t.Fatalf("input mismatch: %+v", base.texts)
	}
}

type stubServexEmbeddingModel struct {
	texts []string
	opts  llm.CallOptions
}

func (m *stubServexEmbeddingModel) EmbedTexts(_ context.Context, texts []string, opts ...llm.CallOption) (*llm.EmbedResponse, error) {
	m.texts = texts
	m.opts = llm.ApplyOptions(opts)
	return &llm.EmbedResponse{Embeddings: [][]float32{{3, 4}}}, nil
}

func TestAsEmbedderAdaptsServexEmbeddingModelToEino(t *testing.T) {
	t.Parallel()

	base := &stubServexEmbeddingModel{}
	embedder, err := AsEmbedder(base)
	if err != nil {
		t.Fatalf("AsEmbedder failed: %v", err)
	}
	vecs, err := embedder.EmbedStrings(t.Context(), []string{"hello"}, embedding.WithModel("emb"))
	if err != nil {
		t.Fatalf("EmbedStrings failed: %v", err)
	}
	if len(vecs) != 1 || len(vecs[0]) != 2 || vecs[0][1] != 4 {
		t.Fatalf("embedding mismatch: %+v", vecs)
	}
	if base.opts.Model != "emb" {
		t.Fatalf("model option mismatch: %+v", base.opts)
	}
}
