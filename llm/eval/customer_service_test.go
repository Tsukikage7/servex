package eval_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/eval"
)

// ──────────────────────────────────────────
// HandoffNeededEvaluator
// ──────────────────────────────────────────

// TestHandoffNeededEvaluator_HighScoreWhenAIStuck AI 用兜底话术时应得高分.
func TestHandoffNeededEvaluator_HighScoreWhenAIStuck(t *testing.T) {
	model := newFixedModel(0.9, "AI 使用了兜底话术")
	ev := eval.HandoffNeededEvaluator(model)
	result, err := ev.Evaluate(context.Background(), eval.EvalInput{
		Question: "我的订单在哪",
		Answer:   "很抱歉，我不知道，请联系客服。",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(result.Scores) != 1 {
		t.Fatalf("期望 1 个 Score，实际=%d", len(result.Scores))
	}
	score := result.Scores[0]
	if score.Name != eval.ScoreHandoffNeeded {
		t.Errorf("期望 Name=%s，实际=%s", eval.ScoreHandoffNeeded, score.Name)
	}
	if score.Value != 0.9 {
		t.Errorf("期望 Value=0.9，实际=%v", score.Value)
	}
	if score.Reason == "" {
		t.Error("期望 Reason 非空")
	}
}

// TestHandoffNeededEvaluator_LowScoreWhenAnswerIsGood 普通正常回答应得低分.
func TestHandoffNeededEvaluator_LowScoreWhenAnswerIsGood(t *testing.T) {
	model := newFixedModel(0.1, "回答具体有效，无需转人工")
	ev := eval.HandoffNeededEvaluator(model)
	result, err := ev.Evaluate(context.Background(), eval.EvalInput{
		Question: "退款多久到账",
		Answer:   "退款会在 3 到 5 个工作日内原路退回。",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if result.Scores[0].Value != 0.1 {
		t.Errorf("期望 Value=0.1，实际=%v", result.Scores[0].Value)
	}
}

// TestHandoffNeededEvaluator_NilModel nil model 返回 ErrNilModel.
func TestHandoffNeededEvaluator_NilModel(t *testing.T) {
	ev := eval.HandoffNeededEvaluator(nil)
	_, err := ev.Evaluate(context.Background(), eval.EvalInput{Question: "q", Answer: "a"})
	if !errors.Is(err, eval.ErrNilModel) {
		t.Errorf("期望 ErrNilModel，实际=%v", err)
	}
}

// TestHandoffNeededEvaluator_EmptyAnswer 空答案返回 ErrEmptyAnswer.
func TestHandoffNeededEvaluator_EmptyAnswer(t *testing.T) {
	model := newFixedModel(0.5, "x")
	ev := eval.HandoffNeededEvaluator(model)
	_, err := ev.Evaluate(context.Background(), eval.EvalInput{Question: "q", Answer: ""})
	if !errors.Is(err, eval.ErrEmptyAnswer) {
		t.Errorf("期望 ErrEmptyAnswer，实际=%v", err)
	}
}

// TestHandoffNeededEvaluator_ScoreClamped 分值自动裁剪到 [0, 1].
func TestHandoffNeededEvaluator_ScoreClamped(t *testing.T) {
	model := newFixedModel(1.5, "out of range")
	ev := eval.HandoffNeededEvaluator(model)
	result, err := ev.Evaluate(context.Background(), eval.EvalInput{Question: "q", Answer: "a"})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if result.Scores[0].Value != 1.0 {
		t.Errorf("期望裁剪到 1.0，实际=%v", result.Scores[0].Value)
	}
}

// TestHandoffNeededEvaluator_PromptContainsCriteria 验证 system prompt 中含关键判据说明.
func TestHandoffNeededEvaluator_PromptContainsCriteria(t *testing.T) {
	var capturedSys string
	model := &mockModel{
		fn: func(msgs []llm.Message) string {
			if len(msgs) > 0 && msgs[0].Role == llm.RoleSystem {
				capturedSys = msgs[0].Content
			}
			return `{"score":0.5,"reason":"ok"}`
		},
	}
	ev := eval.HandoffNeededEvaluator(model)
	_, err := ev.Evaluate(context.Background(), eval.EvalInput{Question: "q", Answer: "a"})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	// 关键词应出现在 prompt 中，以便 LLM 按既定判据评估.
	for _, kw := range []string{"我不知道", "情绪", "转人工"} {
		if !strings.Contains(capturedSys, kw) {
			t.Errorf("system prompt 缺少判据关键词 %q，实际=%q", kw, capturedSys)
		}
	}
}

// ──────────────────────────────────────────
// PolicyComplianceEvaluator
// ──────────────────────────────────────────

// TestPolicyComplianceEvaluator_HighScoreWhenCompliant 合规回答得高分.
func TestPolicyComplianceEvaluator_HighScoreWhenCompliant(t *testing.T) {
	model := newFixedModel(0.95, "未违反任何政策")
	ev := eval.PolicyComplianceEvaluator(model)
	result, err := ev.Evaluate(context.Background(), eval.EvalInput{
		Question: "可以退款吗",
		Answer:   "支持 7 天内无理由退款，请提交工单。",
		Context:  []string{"不得承诺超过 7 天无理由退款", "不得透露用户手机号"},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	score := result.Scores[0]
	if score.Name != eval.ScorePolicyCompliance {
		t.Errorf("期望 Name=%s，实际=%s", eval.ScorePolicyCompliance, score.Name)
	}
	if score.Value != 0.95 {
		t.Errorf("期望 Value=0.95，实际=%v", score.Value)
	}
}

// TestPolicyComplianceEvaluator_LowScoreWhenViolates 违反政策得低分.
func TestPolicyComplianceEvaluator_LowScoreWhenViolates(t *testing.T) {
	model := newFixedModel(0.15, "违反第 1 条政策")
	ev := eval.PolicyComplianceEvaluator(model)
	result, err := ev.Evaluate(context.Background(), eval.EvalInput{
		Question: "可以退款吗",
		Answer:   "当然可以，随时都可以退款，我们 30 天内都支持。",
		Context:  []string{"不得承诺超过 7 天无理由退款"},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if result.Scores[0].Value != 0.15 {
		t.Errorf("期望 Value=0.15，实际=%v", result.Scores[0].Value)
	}
}

// TestPolicyComplianceEvaluator_PromptContainsPolicies 验证 prompt 中按编号列出政策文本.
func TestPolicyComplianceEvaluator_PromptContainsPolicies(t *testing.T) {
	var capturedSys string
	model := &mockModel{
		fn: func(msgs []llm.Message) string {
			if len(msgs) > 0 && msgs[0].Role == llm.RoleSystem {
				capturedSys = msgs[0].Content
			}
			return `{"score":1,"reason":"ok"}`
		},
	}
	ev := eval.PolicyComplianceEvaluator(model)
	_, err := ev.Evaluate(context.Background(), eval.EvalInput{
		Question: "q",
		Answer:   "a",
		Context:  []string{"政策A", "政策B"},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	// 政策文本以及编号前缀应出现在 prompt 中.
	for _, want := range []string{"1. 政策A", "2. 政策B"} {
		if !strings.Contains(capturedSys, want) {
			t.Errorf("system prompt 缺少 %q，实际=%q", want, capturedSys)
		}
	}
}

// TestPolicyComplianceEvaluator_EmptyPoliciesStillWorks Context 为空仍应正常输出 score.
func TestPolicyComplianceEvaluator_EmptyPoliciesStillWorks(t *testing.T) {
	var capturedSys string
	model := &mockModel{
		fn: func(msgs []llm.Message) string {
			if len(msgs) > 0 && msgs[0].Role == llm.RoleSystem {
				capturedSys = msgs[0].Content
			}
			return `{"score":0.7,"reason":"按常识判断合规"}`
		},
	}
	ev := eval.PolicyComplianceEvaluator(model)
	result, err := ev.Evaluate(context.Background(), eval.EvalInput{
		Question: "q",
		Answer:   "a",
		Context:  nil,
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if result.Scores[0].Value != 0.7 {
		t.Errorf("期望 Value=0.7，实际=%v", result.Scores[0].Value)
	}
	// prompt 应含 fallback 提示.
	if !strings.Contains(capturedSys, "未提供具体政策") {
		t.Errorf("空 Context 时 prompt 应含 fallback 说明，实际=%q", capturedSys)
	}
}

// TestPolicyComplianceEvaluator_NilModel nil model 返回 ErrNilModel.
func TestPolicyComplianceEvaluator_NilModel(t *testing.T) {
	ev := eval.PolicyComplianceEvaluator(nil)
	_, err := ev.Evaluate(context.Background(), eval.EvalInput{Question: "q", Answer: "a"})
	if !errors.Is(err, eval.ErrNilModel) {
		t.Errorf("期望 ErrNilModel，实际=%v", err)
	}
}

// TestPolicyComplianceEvaluator_EmptyAnswer 空答案返回 ErrEmptyAnswer.
func TestPolicyComplianceEvaluator_EmptyAnswer(t *testing.T) {
	model := newFixedModel(0.5, "x")
	ev := eval.PolicyComplianceEvaluator(model)
	_, err := ev.Evaluate(context.Background(), eval.EvalInput{Question: "q", Answer: ""})
	if !errors.Is(err, eval.ErrEmptyAnswer) {
		t.Errorf("期望 ErrEmptyAnswer，实际=%v", err)
	}
}

// TestPolicyComplianceEvaluator_TrimsAndSkipsBlankPolicies 空字符串政策应被跳过.
func TestPolicyComplianceEvaluator_TrimsAndSkipsBlankPolicies(t *testing.T) {
	var capturedSys string
	model := &mockModel{
		fn: func(msgs []llm.Message) string {
			if len(msgs) > 0 && msgs[0].Role == llm.RoleSystem {
				capturedSys = msgs[0].Content
			}
			return `{"score":1,"reason":"ok"}`
		},
	}
	ev := eval.PolicyComplianceEvaluator(model)
	_, err := ev.Evaluate(context.Background(), eval.EvalInput{
		Question: "q",
		Answer:   "a",
		Context:  []string{"", "  ", "真政策"},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	// 只应出现 "1. 真政策" 而非 "3. 真政策".
	if !strings.Contains(capturedSys, "1. 真政策") {
		t.Errorf("空字符串政策应被跳过，prompt=%q", capturedSys)
	}
	if strings.Contains(capturedSys, "3. 真政策") {
		t.Errorf("不应保留被跳过政策的原始编号，prompt=%q", capturedSys)
	}
}

// TestCustomerServiceEvaluators_InComposite 验证两个评估器可与现有 composite 组合并发运行.
func TestCustomerServiceEvaluators_InComposite(t *testing.T) {
	model := newFixedModel(0.8, "ok")
	composite := eval.NewCompositeEvaluator(
		eval.HandoffNeededEvaluator(model),
		eval.PolicyComplianceEvaluator(model),
	)
	result, err := composite.Evaluate(context.Background(), eval.EvalInput{
		Question: "q",
		Answer:   "a",
		Context:  []string{"P1"},
	})
	if err != nil {
		t.Fatalf("Composite Evaluate: %v", err)
	}
	names := map[string]bool{}
	for _, s := range result.Scores {
		names[s.Name] = true
	}
	if !names[eval.ScoreHandoffNeeded] || !names[eval.ScorePolicyCompliance] {
		t.Errorf("期望包含 handoff_needed 和 policy_compliance 两个 Score，实际=%+v", result.Scores)
	}
}
