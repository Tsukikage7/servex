package eval_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/eval"
)

// mockModel 测试用模拟模型，通过函数自定义响应内容.
type mockModel struct {
	fn func(msgs []llm.Message) string
}

// Generate 根据 fn 生成响应内容.
func (m *mockModel) Generate(_ context.Context, msgs []llm.Message, _ ...llm.CallOption) (*llm.ChatResponse, error) {
	content := m.fn(msgs)
	return &llm.ChatResponse{Message: llm.AssistantMessage(content)}, nil
}

// Stream 未实现，仅满足接口要求.
func (m *mockModel) Stream(_ context.Context, _ []llm.Message, _ ...llm.CallOption) (llm.StreamReader, error) {
	return nil, fmt.Errorf("not implemented")
}

// newFixedModel 创建始终返回固定 JSON 响应的模拟模型.
func newFixedModel(score float64, reason string) *mockModel {
	return &mockModel{
		fn: func(_ []llm.Message) string {
			return fmt.Sprintf(`{"score":%v,"reason":"%s"}`, score, reason)
		},
	}
}

// TestRelevanceEvaluator 验证相关性评估器正确解析模型响应并返回 Score.
func TestRelevanceEvaluator(t *testing.T) {
	model := newFixedModel(0.9, "回答与问题高度相关")
	ev := eval.RelevanceEvaluator(model)

	result, err := ev.Evaluate(t.Context(), eval.EvalInput{
		Question: "什么是机器学习？",
		Answer:   "机器学习是人工智能的一个子领域。",
	})
	if err != nil {
		t.Fatalf("RelevanceEvaluator 失败: %v", err)
	}
	if len(result.Scores) != 1 {
		t.Fatalf("期望 1 个 Score，得到 %d", len(result.Scores))
	}

	score := result.Scores[0]
	// 验证评估器名称.
	if score.Name != "relevance" {
		t.Errorf("期望 Name=relevance，得到 %q", score.Name)
	}
	// 验证分值.
	if score.Value != 0.9 {
		t.Errorf("期望 Value=0.9，得到 %v", score.Value)
	}
	// 验证理由.
	if score.Reason != "回答与问题高度相关" {
		t.Errorf("期望 Reason=回答与问题高度相关，得到 %q", score.Reason)
	}
}

// TestFaithfulnessEvaluator 验证忠实性评估器正确处理带参考资料的输入.
func TestFaithfulnessEvaluator(t *testing.T) {
	// 验证系统提示中包含参考资料的模拟模型.
	var capturedSysPrompt string
	model := &mockModel{
		fn: func(msgs []llm.Message) string {
			if len(msgs) > 0 && msgs[0].Role == llm.RoleSystem {
				capturedSysPrompt = msgs[0].Content
			}
			return `{"score":0.85,"reason":"回答完全基于参考资料"}`
		},
	}

	contextDocs := []string{"深度学习是机器学习的子领域", "神经网络是深度学习的基础"}
	ev := eval.FaithfulnessEvaluator(model)

	result, err := ev.Evaluate(t.Context(), eval.EvalInput{
		Question: "什么是深度学习？",
		Answer:   "深度学习是机器学习的子领域，使用神经网络。",
		Context:  contextDocs,
	})
	if err != nil {
		t.Fatalf("FaithfulnessEvaluator 失败: %v", err)
	}
	if len(result.Scores) != 1 {
		t.Fatalf("期望 1 个 Score，得到 %d", len(result.Scores))
	}

	score := result.Scores[0]
	if score.Name != "faithfulness" {
		t.Errorf("期望 Name=faithfulness，得到 %q", score.Name)
	}
	if score.Value != 0.85 {
		t.Errorf("期望 Value=0.85，得到 %v", score.Value)
	}

	// 验证系统提示中包含参考资料.
	if capturedSysPrompt == "" {
		t.Error("期望捕获到系统提示")
	}
	for _, doc := range contextDocs {
		if !contains(capturedSysPrompt, doc) {
			t.Errorf("期望系统提示包含参考资料 %q", doc)
		}
	}
}

// TestCoherenceEvaluator 验证连贯性评估器正确解析响应.
func TestCoherenceEvaluator(t *testing.T) {
	model := newFixedModel(0.75, "逻辑清晰，表达流畅")
	ev := eval.CoherenceEvaluator(model)

	result, err := ev.Evaluate(t.Context(), eval.EvalInput{
		Question: "请解释量子计算的原理。",
		Answer:   "量子计算利用量子比特进行并行计算，速度远超传统计算机。",
	})
	if err != nil {
		t.Fatalf("CoherenceEvaluator 失败: %v", err)
	}
	if len(result.Scores) != 1 {
		t.Fatalf("期望 1 个 Score，得到 %d", len(result.Scores))
	}

	score := result.Scores[0]
	if score.Name != "coherence" {
		t.Errorf("期望 Name=coherence，得到 %q", score.Name)
	}
	if score.Value != 0.75 {
		t.Errorf("期望 Value=0.75，得到 %v", score.Value)
	}
	if score.Reason != "逻辑清晰，表达流畅" {
		t.Errorf("期望 Reason=逻辑清晰，表达流畅，得到 %q", score.Reason)
	}
}

// TestCorrectnessEvaluator 验证正确性评估器正确处理带参考答案的输入.
func TestCorrectnessEvaluator(t *testing.T) {
	// 捕获系统提示以验证参考答案被注入.
	var capturedSysPrompt string
	model := &mockModel{
		fn: func(msgs []llm.Message) string {
			if len(msgs) > 0 && msgs[0].Role == llm.RoleSystem {
				capturedSysPrompt = msgs[0].Content
			}
			return `{"score":0.95,"reason":"与参考答案高度一致"}`
		},
	}

	reference := "水的化学式是 H2O，由两个氢原子和一个氧原子组成。"
	ev := eval.CorrectnessEvaluator(model)

	result, err := ev.Evaluate(t.Context(), eval.EvalInput{
		Question:  "水的化学式是什么？",
		Answer:    "水的化学式是 H2O。",
		Reference: reference,
	})
	if err != nil {
		t.Fatalf("CorrectnessEvaluator 失败: %v", err)
	}
	if len(result.Scores) != 1 {
		t.Fatalf("期望 1 个 Score，得到 %d", len(result.Scores))
	}

	score := result.Scores[0]
	if score.Name != "correctness" {
		t.Errorf("期望 Name=correctness，得到 %q", score.Name)
	}
	if score.Value != 0.95 {
		t.Errorf("期望 Value=0.95，得到 %v", score.Value)
	}

	// 验证系统提示中包含参考答案.
	if !contains(capturedSysPrompt, reference) {
		t.Errorf("期望系统提示包含参考答案 %q", reference)
	}
}

// TestCompositeEvaluator 验证组合评估器运行多个评估器并合并所有 Score.
func TestCompositeEvaluator(t *testing.T) {
	relevanceModel := newFixedModel(0.9, "高度相关")
	coherenceModel := newFixedModel(0.8, "逻辑连贯")

	composite := eval.NewCompositeEvaluator(
		eval.RelevanceEvaluator(relevanceModel),
		eval.CoherenceEvaluator(coherenceModel),
	)

	result, err := composite.Evaluate(t.Context(), eval.EvalInput{
		Question: "什么是 Go 语言？",
		Answer:   "Go 是 Google 开发的静态类型编译语言。",
	})
	if err != nil {
		t.Fatalf("CompositeEvaluator 失败: %v", err)
	}

	// 验证结果包含 2 个 Score.
	if len(result.Scores) != 2 {
		t.Fatalf("期望 2 个 Score，得到 %d", len(result.Scores))
	}

	// 收集所有评估器名称.
	names := make(map[string]bool)
	for _, s := range result.Scores {
		names[s.Name] = true
	}
	if !names["relevance"] {
		t.Error("期望结果中包含 relevance 评分")
	}
	if !names["coherence"] {
		t.Error("期望结果中包含 coherence 评分")
	}
}

// TestEvaluator_EmptyAnswer 验证 Answer 为空时返回 ErrEmptyAnswer 错误.
func TestEvaluator_EmptyAnswer(t *testing.T) {
	model := newFixedModel(0.5, "测试")
	evaluators := []struct {
		name string
		ev   eval.Evaluator
	}{
		{"RelevanceEvaluator", eval.RelevanceEvaluator(model)},
		{"FaithfulnessEvaluator", eval.FaithfulnessEvaluator(model)},
		{"CoherenceEvaluator", eval.CoherenceEvaluator(model)},
		{"CorrectnessEvaluator", eval.CorrectnessEvaluator(model)},
	}

	for _, tc := range evaluators {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.ev.Evaluate(t.Context(), eval.EvalInput{
				Question: "测试问题",
				Answer:   "", // 空答案.
			})
			if !errors.Is(err, eval.ErrEmptyAnswer) {
				t.Errorf("%s：期望 ErrEmptyAnswer，得到: %v", tc.name, err)
			}
		})
	}
}

// TestNilModel_ReturnsError 验证 nil model 返回 ErrNilModel.
func TestNilModel_ReturnsError(t *testing.T) {
	evaluators := []struct {
		name string
		ev   eval.Evaluator
	}{
		{"Relevance", eval.RelevanceEvaluator(nil)},
		{"Faithfulness", eval.FaithfulnessEvaluator(nil)},
		{"Coherence", eval.CoherenceEvaluator(nil)},
		{"Correctness", eval.CorrectnessEvaluator(nil)},
	}

	for _, tc := range evaluators {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.ev.Evaluate(t.Context(), eval.EvalInput{
				Question: "q",
				Answer:   "a",
			})
			if !errors.Is(err, eval.ErrNilModel) {
				t.Errorf("expected ErrNilModel, got %v", err)
			}
		})
	}
}

// TestCompositeEvaluator_ConcurrentExecution 验证组合评估器并发执行所有子评估器.
func TestCompositeEvaluator_ConcurrentExecution(t *testing.T) {
	// Use 4 evaluators to test concurrency.
	models := make([]*mockModel, 4)
	evaluators := make([]eval.Evaluator, 4)
	for i := range 4 {
		score := float64(i+1) * 0.2
		reason := fmt.Sprintf("reason-%d", i)
		models[i] = newFixedModel(score, reason)
	}
	evaluators[0] = eval.RelevanceEvaluator(models[0])
	evaluators[1] = eval.FaithfulnessEvaluator(models[1])
	evaluators[2] = eval.CoherenceEvaluator(models[2])
	evaluators[3] = eval.CorrectnessEvaluator(models[3])

	composite := eval.NewCompositeEvaluator(evaluators...)

	result, err := composite.Evaluate(t.Context(), eval.EvalInput{
		Question:  "test question",
		Answer:    "test answer",
		Reference: "ref",
		Context:   []string{"ctx1"},
	})
	if err != nil {
		t.Fatalf("CompositeEvaluator error: %v", err)
	}
	if len(result.Scores) != 4 {
		t.Fatalf("expected 4 scores, got %d", len(result.Scores))
	}
}

// TestCompositeEvaluator_WithError 验证组合评估器中一个子评估器出错.
func TestCompositeEvaluator_WithError(t *testing.T) {
	goodModel := newFixedModel(0.8, "ok")
	composite := eval.NewCompositeEvaluator(
		eval.RelevanceEvaluator(goodModel),
		eval.CoherenceEvaluator(nil), // nil model => ErrNilModel
	)

	_, err := composite.Evaluate(t.Context(), eval.EvalInput{
		Question: "q",
		Answer:   "a",
	})
	if err == nil {
		t.Error("expected error from composite with nil model evaluator")
	}
}

// TestCompositeEvaluator_Empty 验证空组合评估器.
func TestCompositeEvaluator_Empty(t *testing.T) {
	composite := eval.NewCompositeEvaluator()
	result, err := composite.Evaluate(t.Context(), eval.EvalInput{
		Question: "q",
		Answer:   "a",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Scores) != 0 {
		t.Errorf("expected 0 scores, got %d", len(result.Scores))
	}
}

// TestEvaluator_EmptyQuestion 验证空问题不会报错（只要 answer 非空）.
func TestEvaluator_EmptyQuestion(t *testing.T) {
	model := newFixedModel(0.5, "ok")
	ev := eval.RelevanceEvaluator(model)

	result, err := ev.Evaluate(t.Context(), eval.EvalInput{
		Question: "",
		Answer:   "some answer",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Scores) != 1 {
		t.Errorf("expected 1 score, got %d", len(result.Scores))
	}
}

// TestEvaluator_ScoreClamping 验证分值被归一化到 [0, 1].
func TestEvaluator_ScoreClamping(t *testing.T) {
	tests := []struct {
		name     string
		rawScore float64
		want     float64
	}{
		{"above 1", 1.5, 1.0},
		{"below 0", -0.5, 0.0},
		{"normal", 0.7, 0.7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := newFixedModel(tt.rawScore, "test")
			ev := eval.RelevanceEvaluator(model)
			result, err := ev.Evaluate(t.Context(), eval.EvalInput{
				Question: "q",
				Answer:   "a",
			})
			if err != nil {
				t.Fatalf("error: %v", err)
			}
			if result.Scores[0].Value != tt.want {
				t.Errorf("expected score %v, got %v", tt.want, result.Scores[0].Value)
			}
		})
	}
}

// TestWithCallOptions 验证选项应用不会 panic.
func TestWithCallOptions(t *testing.T) {
	model := newFixedModel(0.8, "ok")
	ev := eval.RelevanceEvaluator(model, eval.WithCallOptions())
	_, err := ev.Evaluate(t.Context(), eval.EvalInput{
		Question: "q",
		Answer:   "a",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// contains 检查字符串 s 是否包含子字符串 sub.
func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || stringContains(s, sub))
}

// stringContains 简单字符串包含检查.
func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
