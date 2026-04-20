# llm/safety/moderation

`github.com/Tsukikage7/servex/llm/safety/moderation` — 内容审核，按类别检测有害内容，支持 LLM 审核、关键词审核及组合审核。

## 核心类型

- `Moderator` — 审核器接口，方法包括 `Moderate(ctx, text)` 和 `ModerateMessages(ctx, messages)`
- `Result` — 审核结果，包含 Flagged（是否违规）、Categories（各类别命中）、Scores（各类别分数）、Reason
- `Category` — 审核类别，内置 `violence`、`sexual`、`hate`、`self_harm`、`dangerous`、`political`、`spam`
- `NewLLMModerator(model, opts...)` — 基于 LLM 的审核器，可配置分数阈值和检测类别子集
- `NewKeywordModerator(rules)` — 基于关键词匹配的快速审核器（rules 为 Category -> 关键词列表的映射）
- `NewCompositeModerator(...)` — 组合审核器，关键词先行短路，避免不必要的 LLM 调用
- `WithThreshold(t)` — 设置触发标记的分数阈值（默认 0.7）
- `WithCategories(cats...)` — 设置需检测的类别子集

## 使用示例

```go
import "github.com/Tsukikage7/servex/llm/safety/moderation"

// LLM 审核器
mod := moderation.NewLLMModerator(myModel,
    moderation.WithThreshold(0.8),
    moderation.WithCategories(moderation.CategoryViolence, moderation.CategoryHate),
)
result, err := mod.Moderate(ctx, "用户输入的文本")
if result.Flagged {
    fmt.Printf("内容违规: %s\n", result.Reason)
}

// 组合审核（关键词快速过滤 + LLM 深度审核）
composite := moderation.NewCompositeModerator(
    moderation.NewKeywordModerator(map[moderation.Category][]string{
        moderation.CategoryViolence: {"打架", "伤害"},
    }),
    moderation.NewLLMModerator(myModel),
)
result, _ = composite.Moderate(ctx, "需要审核的内容")
```

## StreamModerator（流式边生成边审）

`StreamModerator` 包装 `llm.StreamReader`，在 LLM 流式生成过程中按字符数阈值或时间间隔触发一次底层 `Moderator` 审核；命中违规时通过 `OnFlagged` 回调，并让后续 `Recv` 立即返回 `io.EOF`，同时关闭原流。

```go
import "github.com/Tsukikage7/servex/llm/safety/moderation"

sm := moderation.NewStreamModerator(
    myModerator,
    moderation.WithChunkChars(200),                  // 每累积 200 字符触发一次（默认 200）
    moderation.WithChunkInterval(500*time.Millisecond), // 或每 500ms 触发一次（默认 500ms）
    moderation.WithOnFlagged(func(r *moderation.Result) {
        // 记录、告警或回调客户端
    }),
)

reader, _ := model.Stream(ctx, messages)
wrapped := sm.Wrap(reader)
defer wrapped.Close()

for {
    chunk, err := wrapped.Recv()
    if err == io.EOF { break }
    if err != nil { return err }
    // 输出 chunk.Delta
}
```

语义要点：
- 审核在独立 goroutine 中异步执行，不阻塞 `Recv`。
- 同一时刻只有一次审核在进行，新满足阈值的数据会在上一次结束后下一轮触发。
- 命中后缓冲数据不再送审，原 `reader` 在 `Close()` 时关闭。
- 底层 `Moderator` 为 `nil` 时直接透传,不做审核。
