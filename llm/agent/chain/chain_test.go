package chain

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Tsukikage7/servex/llm"
	"github.com/Tsukikage7/servex/llm/prompt"
)

// mockModel 用于测试的模拟 ChatModel，根据输入消息内容返回可预测的响应.
type mockModel struct {
	fn func(msgs []llm.Message) string
}

// Generate 实现 llm.ChatModel.
func (m *mockModel) Generate(ctx context.Context, msgs []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
	content := m.fn(msgs)
	return &llm.ChatResponse{
		Message: llm.AssistantMessage(content),
		Usage:   llm.Usage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
	}, nil
}

// Stream 实现 llm.ChatModel.
func (m *mockModel) Stream(ctx context.Context, msgs []llm.Message, opts ...llm.CallOption) (llm.StreamReader, error) {
	return nil, errors.New("not implemented")
}

// 编译期接口断言.
var _ llm.ChatModel = (*mockModel)(nil)

// TestChain_SingleStep 测试单步链执行，验证输出正确.
func TestChain_SingleStep(t *testing.T) {
	model := &mockModel{fn: func(msgs []llm.Message) string {
		return "摘要：" + msgs[0].Content
	}}

	tmpl := prompt.MustNew(llm.RoleUser, "请总结：{{.}}")
	c := New(WithModel(model))
	c.AddStep(Step{Name: "summarize", Prompt: tmpl})

	result, err := c.Run(t.Context(), "这是一段很长的文章")
	if err != nil {
		t.Fatalf("Run 返回错误: %v", err)
	}

	if len(result.Steps) != 1 {
		t.Fatalf("期望 1 个步骤结果，得到 %d", len(result.Steps))
	}
	if result.Steps[0].Name != "summarize" {
		t.Errorf("步骤名称错误: %s", result.Steps[0].Name)
	}
	if !strings.Contains(result.Output, "摘要：") {
		t.Errorf("输出内容错误: %s", result.Output)
	}
	// 验证 usage 累加
	if result.Usage.TotalTokens != 30 {
		t.Errorf("期望 TotalTokens=30，得到 %d", result.Usage.TotalTokens)
	}
}

// TestChain_MultiStep 测试两步链（摘要 → 翻译），验证两步结果均正确.
func TestChain_MultiStep(t *testing.T) {
	summarizeModel := &mockModel{fn: func(msgs []llm.Message) string {
		return "summary of: " + msgs[0].Content
	}}
	translateModel := &mockModel{fn: func(msgs []llm.Message) string {
		return "translated: " + msgs[0].Content
	}}

	summarizeTmpl := prompt.MustNew(llm.RoleUser, "请总结：{{.}}")
	translateTmpl := prompt.MustNew(llm.RoleUser, "请翻译上一步结果：{{.Previous}}")

	c := New()
	c.AddStep(Step{Name: "summarize", Prompt: summarizeTmpl, Model: summarizeModel})
	c.AddStep(Step{Name: "translate", Prompt: translateTmpl, Model: translateModel})

	result, err := c.Run(t.Context(), "原始文章内容")
	if err != nil {
		t.Fatalf("Run 返回错误: %v", err)
	}

	if len(result.Steps) != 2 {
		t.Fatalf("期望 2 个步骤结果，得到 %d", len(result.Steps))
	}

	// 验证第一步
	step0 := result.Steps[0]
	if step0.Name != "summarize" {
		t.Errorf("步骤 0 名称错误: %s", step0.Name)
	}
	if !strings.Contains(step0.Output, "summary of:") {
		t.Errorf("步骤 0 输出错误: %s", step0.Output)
	}

	// 验证第二步
	step1 := result.Steps[1]
	if step1.Name != "translate" {
		t.Errorf("步骤 1 名称错误: %s", step1.Name)
	}
	if !strings.Contains(step1.Output, "translated:") {
		t.Errorf("步骤 1 输出错误: %s", step1.Output)
	}

	// 最终输出为最后一步
	if result.Output != step1.Output {
		t.Errorf("Result.Output 应等于最后一步输出")
	}

	// 验证 usage 累加（两步各 30 tokens）
	if result.Usage.TotalTokens != 60 {
		t.Errorf("期望 TotalTokens=60，得到 %d", result.Usage.TotalTokens)
	}
}

// TestChain_WithParser 测试带 Parser 的步骤，验证 Parsed 字段正确设置.
func TestChain_WithParser(t *testing.T) {
	model := &mockModel{fn: func(msgs []llm.Message) string {
		return `{"score": 95}`
	}}

	type ScoreResult struct {
		Raw string
	}

	tmpl := prompt.MustNew(llm.RoleUser, "评分以下内容：{{.}}")
	c := New(WithModel(model))
	c.AddStep(Step{
		Name:   "score",
		Prompt: tmpl,
		Parser: func(response string) (any, error) {
			// 简单解析：直接包装原始字符串
			return ScoreResult{Raw: response}, nil
		},
	})

	result, err := c.Run(t.Context(), "这是需要评分的内容")
	if err != nil {
		t.Fatalf("Run 返回错误: %v", err)
	}

	if result.Parsed == nil {
		t.Fatal("期望 Parsed 不为 nil")
	}
	sr, ok := result.Parsed.(ScoreResult)
	if !ok {
		t.Fatalf("Parsed 类型错误，得到 %T", result.Parsed)
	}
	if !strings.Contains(sr.Raw, "score") {
		t.Errorf("Parsed.Raw 内容错误: %s", sr.Raw)
	}

	// 步骤结果中的 Parsed 也应正确
	if result.Steps[0].Parsed == nil {
		t.Error("步骤 Parsed 不应为 nil")
	}
}

// TestChain_OnStep 测试步骤回调，验证每个步骤都会触发回调.
func TestChain_OnStep(t *testing.T) {
	model := &mockModel{fn: func(msgs []llm.Message) string {
		return "response"
	}}

	tmpl1 := prompt.MustNew(llm.RoleUser, "步骤一：{{.}}")
	tmpl2 := prompt.MustNew(llm.RoleUser, "步骤二：{{.Previous}}")

	var events []StepEvent
	handler := func(ctx context.Context, event StepEvent) {
		events = append(events, event)
	}

	c := New(WithModel(model), WithOnStep(handler))
	c.AddStep(Step{Name: "step1", Prompt: tmpl1})
	c.AddStep(Step{Name: "step2", Prompt: tmpl2})

	_, err := c.Run(t.Context(), "input")
	if err != nil {
		t.Fatalf("Run 返回错误: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("期望触发 2 次回调，实际触发 %d 次", len(events))
	}
	if events[0].StepName != "step1" || events[0].StepIndex != 0 {
		t.Errorf("事件 0 错误: name=%s index=%d", events[0].StepName, events[0].StepIndex)
	}
	if events[1].StepName != "step2" || events[1].StepIndex != 1 {
		t.Errorf("事件 1 错误: name=%s index=%d", events[1].StepName, events[1].StepIndex)
	}
	// 验证耗时已记录
	if events[0].Duration <= 0 {
		t.Error("事件 0 的 Duration 应大于 0")
	}
}

// TestChain_Validation 测试参数校验（无模型、无步骤）.
func TestChain_Validation(t *testing.T) {
	t.Run("无步骤", func(t *testing.T) {
		c := New(WithModel(&mockModel{fn: func(msgs []llm.Message) string { return "" }}))
		_, err := c.Run(t.Context(), "input")
		if !errors.Is(err, ErrNoSteps) {
			t.Errorf("期望 ErrNoSteps，得到 %v", err)
		}
	})

	t.Run("无模型", func(t *testing.T) {
		tmpl := prompt.MustNew(llm.RoleUser, "{{.}}")
		c := New()
		c.AddStep(Step{Name: "step", Prompt: tmpl})
		_, err := c.Run(t.Context(), "input")
		if !errors.Is(err, ErrNoModel) {
			t.Errorf("期望 ErrNoModel，得到 %v", err)
		}
	})

	t.Run("步骤 Prompt 为 nil", func(t *testing.T) {
		c := New(WithModel(&mockModel{fn: func(msgs []llm.Message) string { return "" }}))
		c.AddStep(Step{Name: "bad-step", Prompt: nil})
		_, err := c.Run(t.Context(), "input")
		if !errors.Is(err, ErrNilPrompt) {
			t.Errorf("期望 ErrNilPrompt，得到 %v", err)
		}
	})
}
