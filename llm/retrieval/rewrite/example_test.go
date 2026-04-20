package rewrite_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/retrieval/rewrite"
)

// fixedChatModel 固定返回指定文本的 mock chat model，用于 Example 演示.
type fixedChatModel struct{ reply string }

func (m *fixedChatModel) Generate(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{Message: llm.AssistantMessage(m.reply)}, nil
}

// Stream 本 Example 未使用；返回固定错误.
func (m *fixedChatModel) Stream(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (llm.StreamReader, error) {
	return nil, errors.New("fixedChatModel: Stream unused")
}

// ExampleNewHistoryAwareRewriter 展示基于对话历史改写带代词的问题.
func ExampleNewHistoryAwareRewriter() {
	model := &fixedChatModel{reply: "VPS 产品怎么退款"}
	r, _ := rewrite.NewHistoryAwareRewriter(model)

	history := []llm.Message{
		llm.UserMessage("A 产品怎么退款"),
		llm.AssistantMessage("支持 7 天内无理由"),
	}
	out, _ := r.Rewrite(context.Background(), "那 B 呢", history)
	fmt.Println(out)
	// Output:
	// VPS 产品怎么退款
}

// ExampleNewHyDERewriter 展示 HyDE：LLM 生成假设性答案作为检索查询.
func ExampleNewHyDERewriter() {
	model := &fixedChatModel{reply: "VPS 支持 7 天内无理由退款，提交工单即可申请。"}
	r, _ := rewrite.NewHyDERewriter(model)

	out, _ := r.Rewrite(context.Background(), "VPS 可以退款吗", nil)
	fmt.Println(out)
	// Output:
	// VPS 支持 7 天内无理由退款，提交工单即可申请。
}
