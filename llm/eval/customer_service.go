package eval

import (
	"fmt"
	"strings"

	"github.com/Tsukikage7/servex/v2/llm"
)

// 客服域评估器名称常量.
const (
	// ScoreHandoffNeeded 评分维度：是否应转人工.
	ScoreHandoffNeeded = "handoff_needed"
	// ScorePolicyCompliance 评分维度：是否合规.
	ScorePolicyCompliance = "policy_compliance"
)

// handoffNeededPrompt HandoffNeeded 评估器的 system prompt.
//
// LLM 需要判断：给定的客服问答是否应当升级为人工服务.
// 典型触发场景：AI 明显无能为力（"我不知道"/"请联系客服"）、用户情绪激烈、
// 话题涉及政策 / 法务 / 退款纠纷等边界不清的领域.
//
// 输出约定与 eval 其余评估器一致：{"score": 0-1, "reason": "..."}，
// score 越高表示越应该转人工.
const handoffNeededPrompt = `你是一位专业的 AI 客服质量评估专家.
判断以下客服问答对是否"应当转人工客服".
若命中任一情形则应给高分（越高越应转人工）：
  1. AI 回答含"我不知道 / 无法回答 / 请联系客服 / 我没有这方面的信息"等兜底话术
  2. 用户流露强烈负面情绪（愤怒、威胁、反复抱怨）
  3. 话题涉及退款纠纷 / 法务 / 隐私 / 账户安全等边界不清领域
  4. AI 回答与问题明显不相关或答非所问

只输出 JSON，不要任何前缀或后缀：
{"score": 0-1, "reason": "..."}`

// HandoffNeededEvaluator 创建"是否应转人工"评估器.
//
// 输入：EvalInput.Question（必填）、EvalInput.Answer（必填）.
// 输出：单 Score：Name="handoff_needed"，Value 在 [0, 1]，越高越应转人工，Reason 为 LLM 给的理由.
//
// model 为 nil 时返回的 Evaluator 会在 Evaluate 时返回 ErrNilModel.
func HandoffNeededEvaluator(model llm.ChatModel, opts ...Option) Evaluator {
	if model == nil {
		return &errEvaluator{err: ErrNilModel}
	}
	o := applyOptions(opts)
	return &llmEvaluator{
		name:        ScoreHandoffNeeded,
		model:       model,
		opts:        o,
		buildPrompt: func(_ EvalInput) string { return handoffNeededPrompt },
	}
}

// policyComplianceBasePrompt PolicyCompliance 评估器的 system prompt 基座.
//
// 输出语义：score 越高越合规（1.0 = 完全合规，0.0 = 严重违反）.
// LLM 需要引用 EvalInput.Context 中的政策文本条目，把每条政策作为判据.
//
// Context 可为空——空时 LLM 仅按常识判断（仍输出同结构 JSON）.
const policyComplianceBasePrompt = `你是一位专业的 AI 客服合规审查专家.
以下给出若干条客服政策。请检查【回答】是否违反任何一条政策.
score 越高越合规（1.0=完全合规，0.0=严重违反）.

# 政策条目
%s

只输出 JSON，不要任何前缀或后缀：
{"score": 0-1, "reason": "..."}`

// PolicyComplianceEvaluator 创建政策合规性评估器.
//
// 输入：EvalInput.Question、EvalInput.Answer（必填）、EvalInput.Context（政策文本列表）.
// Context 的每个元素视为一条独立政策；LLM 需要判断 Answer 是否违反任一条.
//
// 输出：单 Score：Name="policy_compliance"，Value 在 [0, 1]，越高越合规.
// model 为 nil 时返回的 Evaluator 会在 Evaluate 时返回 ErrNilModel.
func PolicyComplianceEvaluator(model llm.ChatModel, opts ...Option) Evaluator {
	if model == nil {
		return &errEvaluator{err: ErrNilModel}
	}
	o := applyOptions(opts)
	return &llmEvaluator{
		name:  ScorePolicyCompliance,
		model: model,
		opts:  o,
		buildPrompt: func(input EvalInput) string {
			return fmt.Sprintf(policyComplianceBasePrompt, formatPolicies(input.Context))
		},
	}
}

// formatPolicies 把政策列表格式化为带编号的多行字符串.
// 空列表时返回 "（未提供具体政策，请按常识判断）"，确保 prompt 仍完整可读.
func formatPolicies(policies []string) string {
	nonEmpty := make([]string, 0, len(policies))
	for _, p := range policies {
		p = strings.TrimSpace(p)
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	if len(nonEmpty) == 0 {
		return "（未提供具体政策，请按常识判断）"
	}
	var b strings.Builder
	for i, p := range nonEmpty {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%d. %s", i+1, p)
	}
	return b.String()
}
