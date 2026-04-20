# llm/eval

`github.com/Tsukikage7/servex/llm/eval` — LLM 输出质量评估框架，支持相关性、忠实性、连贯性、正确性等多维度评估。

## 核心类型

- `Evaluator` — 评估器接口，方法为 `Evaluate(ctx, EvalInput) (*EvalResult, error)`
- `EvalInput` — 评估输入，包含 Question、Answer、Reference（参考答案）、Context（参考资料列表）
- `EvalResult` — 评估结果，包含 `[]Score`（各维度评分）
- `Score` — 单项评分，包含 Name、Value（0.0-1.0）、Reason
- `RelevanceEvaluator(model)` — 创建相关性评估器
- `FaithfulnessEvaluator(model)` — 创建忠实性评估器（基于参考资料）
- `CoherenceEvaluator(model)` — 创建连贯性评估器
- `CorrectnessEvaluator(model)` — 创建正确性评估器（基于参考答案）
- `HandoffNeededEvaluator(model)` — 创建"是否应转人工"评估器（客服域；score 越高越应转人工）
- `PolicyComplianceEvaluator(model)` — 创建政策合规评估器（客服域；score 越高越合规），`EvalInput.Context` 传入政策文本列表
- `NewCompositeEvaluator(...)` — 创建组合评估器，并发运行所有子评估器并合并结果

## 使用示例

```go
import "github.com/Tsukikage7/servex/llm/eval"

// 单一维度评估
relevance := eval.RelevanceEvaluator(myModel)
result, err := relevance.Evaluate(ctx, eval.EvalInput{
    Question: "什么是机器学习？",
    Answer:   "机器学习是一种让计算机从数据中学习的技术。",
})
fmt.Printf("相关性: %.2f，理由：%s\n", result.Scores[0].Value, result.Scores[0].Reason)

// 多维度组合评估
composite := eval.NewCompositeEvaluator(
    eval.RelevanceEvaluator(myModel),
    eval.CoherenceEvaluator(myModel),
    eval.CorrectnessEvaluator(myModel),
)
multiResult, _ := composite.Evaluate(ctx, eval.EvalInput{
    Question:  "首都是哪里？",
    Answer:    "北京是中国的首都。",
    Reference: "中国的首都是北京。",
})
for _, s := range multiResult.Scores {
    fmt.Printf("%s: %.2f\n", s.Name, s.Value)
}
```

## 客服域评估器

`HandoffNeededEvaluator` 与 `PolicyComplianceEvaluator` 针对 AI 客服场景：

```go
// 是否应转人工：AI 兜底话术、用户情绪、边界话题等情形得高分
handoff := eval.HandoffNeededEvaluator(myModel)
result, _ := handoff.Evaluate(ctx, eval.EvalInput{
    Question: "我的订单在哪",
    Answer:   "很抱歉，我不知道，请联系客服。",
})
// result.Scores[0].Name == "handoff_needed"，Value 越高越建议转人工

// 政策合规：Context 传入政策文本列表，LLM 判断 Answer 是否违反任一条
policy := eval.PolicyComplianceEvaluator(myModel)
result, _ = policy.Evaluate(ctx, eval.EvalInput{
    Question: "可以退款吗",
    Answer:   "当然可以，随时都可以退款，我们 30 天内都支持。",
    Context: []string{
        "不得承诺超过 7 天无理由退款",
        "不得透露用户手机号",
    },
})
// result.Scores[0].Name == "policy_compliance"，Value 越高越合规（1=完全合规，0=严重违反）
```

两个评估器与其他评估器可直接组合进 `NewCompositeEvaluator`，并发跑、合并 `Scores`。

