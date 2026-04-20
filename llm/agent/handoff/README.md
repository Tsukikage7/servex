# llm/agent/handoff

`github.com/Tsukikage7/servex/v2/llm/agent/handoff` — AI 客服人工接管检测与 Hook 触发子包。通过 `Detector` 抽象检测策略、`Hook` 抽象副作用（写工单/发消息队列/调 webhook），可组合成单一检测流。

## 核心类型

- `Detector` — 检测器接口，方法为 `Detect(ctx, DetectInput) (*Signal, error)`
- `Hook` — 命中后的副作用触发器，方法为 `Fire(ctx, *Signal, DetectInput) error`
- `Signal` — 检测信号，字段 `Should`（是否转人工）、`Reason`（原因标签）、`Meta`（扩展元数据）
- `DetectInput` — 检测输入：`Question`、`Answer`、`History`、`RetryCount`、`LastScore`

### 内置 Detector

- `NewKeywordDetector(keywords)` — Question/Answer 包含任一关键词即命中，`Meta["matched"]`/`Meta["source"]` 记录命中细节
- `NewLowConfidenceDetector(threshold)` — RAG 召回最高 `LastScore < threshold` 命中；`LastScore <= 0` 视为未提供，跳过
- `NewRetryDetector(maxRetry)` — 用户在 session 内重复提问次数 `>= maxRetry` 命中
- `NewLLMDetector(model, opts...)` — LLM 判定是否有强烈情绪或明确转人工意图；`model` 为 `nil` 时返回 `ErrNilModel`
- `NewCompositeDetector(detectors...)` — 顺序执行子 Detector，**任一命中即短路返回**；任一子 Detector 报错立即返回 err；支持 `nil` 子 Detector（被跳过）

### 内置 Hook

- `NewFuncHook(fn)` — 任意函数包装为 Hook；`fn` 为 `nil` 时返回 `ErrNilHookFunc`
- `NewWebhookHook(url, opts...)` — 命中时 POST JSON 到指定 URL；`WithTimeout`/`WithHTTPClient`/`WithHeader` 调配

### Reason 常量

- `ReasonKeyword` = `"keyword"`
- `ReasonLowConfidence` = `"low_confidence"`
- `ReasonRetryExceeded` = `"retry_exceeded"`
- `ReasonLLMDetected` = `"llm_detected"`

## 行为约定

- 所有 Detector：未命中返回 `&Signal{Should:false}`（非 nil），方便调用方直接读 `Meta`
- `CompositeDetector`：顺序依赖传入顺序，前面命中后面不再执行
- `LLMDetector`：LLM 调用失败或 JSON 解析失败时返回 `(未命中 Signal, error)`——保守地**不**自动转人工
- `WebhookHook`：非 2xx 状态码返回错误；`Content-Type: application/json`；默认 5s 超时

## 使用示例

### 组合检测 + FuncHook

```go
import "github.com/Tsukikage7/servex/v2/llm/agent/handoff"

detector := handoff.NewCompositeDetector(
    handoff.NewKeywordDetector([]string{"转人工", "投诉", "真人"}),
    handoff.NewLowConfidenceDetector(0.35),
    handoff.NewRetryDetector(3),
)

hook, _ := handoff.NewFuncHook(func(ctx context.Context, sig *handoff.Signal, in handoff.DetectInput) error {
    // 写工单表、发消息队列等
    return createTicket(ctx, sig.Reason, in.Question, in.Answer)
})

// 在 chat handler 末尾
sig, _ := detector.Detect(ctx, handoff.DetectInput{
    Question:   userQuestion,
    Answer:     answer,
    History:    history,
    RetryCount: retryCount,
    LastScore:  topRAGScore,
})
if sig.Should {
    _ = hook.Fire(ctx, sig, in)
    answer = "已为您转接人工客服，客服会尽快回复。"
}
```

### Webhook Hook

```go
hook := handoff.NewWebhookHook("https://ops.example.com/ai-handoff",
    handoff.WithTimeout(3*time.Second),
    handoff.WithHeader("Authorization", "Bearer "+token),
)
_ = hook.Fire(ctx, sig, input)
```

Webhook POST body 形如：

```json
{
  "should": true,
  "reason": "keyword",
  "meta": {"matched": "转人工", "source": "question"},
  "question": "我要转人工",
  "history": [...],
  "retry_count": 1,
  "last_score": 0.3
}
```

### LLM Detector 自定义 prompt

```go
d, err := handoff.NewLLMDetector(chatModel,
    handoff.WithLLMSystemPrompt(`你判断用户是否想投诉。只输出 {"should_handoff":bool,"reason":string}`),
    handoff.WithLLMMaxTokens(100),
)
```

> **提示：** 自定义 prompt 必须保留 `{"should_handoff": bool, "reason": string}` 输出格式，否则 JSON 解析会失败。
