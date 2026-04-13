package tokenizer

import (
	"strings"
	"testing"

	"github.com/Tsukikage7/servex/v2/llm"
)

// TestEstimateTokenizer_English 测试纯英文文本的 Token 估算.
// "Hello, World!" 共 13 个 ASCII 字符，按 4 chars/token 估算约 4 tokens（向上取整 ceil(13/4)=4）.
func TestEstimateTokenizer_English(t *testing.T) {
	tok := NewEstimateTokenizer()
	text := "Hello, World!"
	got := tok.Count(text)
	// ceil(13 / 4.0) = 4
	want := 4
	if got != want {
		t.Errorf("Count(%q) = %d，期望 %d", text, got, want)
	}
}

// TestEstimateTokenizer_Chinese 测试纯中文文本的 Token 估算.
// "你好世界" 共 4 个 CJK 字符，按 1.5 chars/token 估算约 3 tokens（向上取整 ceil(4/1.5)=3）.
func TestEstimateTokenizer_Chinese(t *testing.T) {
	tok := NewEstimateTokenizer()
	text := "你好世界"
	got := tok.Count(text)
	// ceil(4 / 1.5) = ceil(2.667) = 3
	want := 3
	if got != want {
		t.Errorf("Count(%q) = %d，期望 %d", text, got, want)
	}
}

// TestEstimateTokenizer_Mixed 测试中英混合文本的 Token 估算.
// "Hello 你好" = 6 ASCII + 2 CJK，ASCII: ceil(6/4)=2，CJK: ceil(2/1.5)=2，合计约 4.
func TestEstimateTokenizer_Mixed(t *testing.T) {
	tok := NewEstimateTokenizer()
	text := "Hello 你好"
	got := tok.Count(text)
	// 6 ASCII chars: 6 * (1000/4) = 1500; 2 CJK chars: 2 * (1000/1.5≈667) = 1334
	// total costX1000 = 2834, ceil(2834/1000) = 3
	if got <= 0 {
		t.Errorf("Count(%q) = %d，期望大于 0", text, got)
	}
	// 合理范围：2~5 tokens
	if got < 2 || got > 5 {
		t.Errorf("Count(%q) = %d，期望在 [2, 5] 范围内", text, got)
	}
}

// TestCountMessages 测试消息列表的 Token 总量计算.
// 3 条消息，每条内容 4 个 ASCII 字符（≈1 token），每条固定开销 4 tokens，合计 3*(1+4)=15.
func TestCountMessages(t *testing.T) {
	tok := NewEstimateTokenizer()
	messages := []llm.Message{
		llm.UserMessage("test"),      // 4 ASCII = 1 token + 4 overhead = 5
		llm.AssistantMessage("word"), // 4 ASCII = 1 token + 4 overhead = 5
		llm.SystemMessage("sys!"),    // 4 ASCII = 1 token + 4 overhead = 5
	}
	got := tok.CountMessages(messages)
	want := 15
	if got != want {
		t.Errorf("CountMessages() = %d，期望 %d", got, want)
	}
}

// TestTruncate 测试将较长文本截断至指定 Token 数.
func TestTruncate(t *testing.T) {
	tok := NewEstimateTokenizer()
	// 40 个 ASCII 字符 ≈ 10 tokens
	text := strings.Repeat("a", 40)
	maxTokens := 10
	result := tok.Truncate(text, maxTokens)

	got := tok.Count(result)
	if got > maxTokens {
		t.Errorf("Truncate 后 Token 数 %d 超过限制 %d，截断结果: %q", got, maxTokens, result)
	}
	if len(result) == 0 {
		t.Error("Truncate 结果不应为空字符串")
	}
}

// TestFitsContext 测试上下文窗口适配判断.
func TestFitsContext(t *testing.T) {
	// 构造占用约 5 tokens 内容的消息（含 overhead 约 9 tokens）
	msgs := []llm.Message{
		llm.UserMessage("Hello"), // 5 ASCII = 2 tokens + 4 overhead = 6
	}

	// 高于实际 token 数，应返回 true
	if !FitsContext(msgs, 100) {
		t.Error("FitsContext(msgs, 100) 应返回 true")
	}

	// 低于实际 token 数，应返回 false
	if FitsContext(msgs, 1) {
		t.Error("FitsContext(msgs, 1) 应返回 false")
	}
}

// TestTruncateToFit 测试消息列表截断：4 条消息，限制 50 tokens，验证系统消息被保留.
func TestTruncateToFit(t *testing.T) {
	// 构造 4 条消息：1 条系统消息 + 3 条非系统消息（较长内容）
	longContent := strings.Repeat("abcdefghij", 5) // 50 ASCII chars ≈ 13 tokens
	messages := []llm.Message{
		llm.SystemMessage("You are a helpful assistant."), // 系统消息，应被保留
		llm.UserMessage(longContent),
		llm.AssistantMessage(longContent),
		llm.UserMessage(longContent),
	}

	maxTokens := 50
	result := TruncateToFit(messages, maxTokens)

	// 验证系统消息被保留
	hasSystem := false
	for _, msg := range result {
		if msg.Role == llm.RoleSystem {
			hasSystem = true
			break
		}
	}
	if !hasSystem {
		t.Error("TruncateToFit 应保留系统消息")
	}

	// 验证截断后总 Token 数不超过限制
	total := EstimateMessageTokens(result)
	if total > maxTokens {
		t.Errorf("TruncateToFit 后 Token 数 %d 超过限制 %d", total, maxTokens)
	}

	// 验证结果不为空
	if len(result) == 0 {
		t.Error("TruncateToFit 结果不应为空")
	}
}

// TestHelperFunctions 测试包级辅助函数 EstimateTokens 和 EstimateMessageTokens.
func TestHelperFunctions(t *testing.T) {
	// 测试 EstimateTokens
	text := "Hello, World!"
	tokens := EstimateTokens(text)
	if tokens <= 0 {
		t.Errorf("EstimateTokens(%q) = %d，期望大于 0", text, tokens)
	}

	// 中文文本
	chineseText := "你好世界"
	chineseTokens := EstimateTokens(chineseText)
	if chineseTokens <= 0 {
		t.Errorf("EstimateTokens(%q) = %d，期望大于 0", chineseText, chineseTokens)
	}

	// 测试 EstimateMessageTokens
	messages := []llm.Message{
		llm.UserMessage("Hello"),
		llm.AssistantMessage("Hi"),
	}
	msgTokens := EstimateMessageTokens(messages)
	if msgTokens <= 0 {
		t.Errorf("EstimateMessageTokens() = %d，期望大于 0", msgTokens)
	}

	// 消息 Token 总数应大于各单条内容的 Token 数之和（因为有固定开销）
	contentTokens := EstimateTokens("Hello") + EstimateTokens("Hi")
	if msgTokens <= contentTokens {
		t.Errorf("EstimateMessageTokens() = %d 应大于纯内容 Token 数 %d（需含固定开销）",
			msgTokens, contentTokens)
	}
}
