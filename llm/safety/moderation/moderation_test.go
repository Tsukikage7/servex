package moderation_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Tsukikage7/servex/llm"
	"github.com/Tsukikage7/servex/llm/safety/moderation"
)

// mockModel 测试用模拟模型，通过函数自定义响应内容.
type mockModel struct {
	// called 记录模型是否被调用，用于验证短路逻辑.
	called bool
	// fn 根据输入消息返回响应文本.
	fn func(msgs []llm.Message) string
}

// Generate 调用 fn 生成响应，并记录调用次数.
func (m *mockModel) Generate(_ context.Context, msgs []llm.Message, _ ...llm.CallOption) (*llm.ChatResponse, error) {
	m.called = true
	content := m.fn(msgs)
	return &llm.ChatResponse{Message: llm.AssistantMessage(content)}, nil
}

// Stream 未实现，仅满足接口要求.
func (m *mockModel) Stream(_ context.Context, _ []llm.Message, _ ...llm.CallOption) (llm.StreamReader, error) {
	return nil, fmt.Errorf("not implemented")
}

// newScoreModel 创建返回指定各类别分数的模拟模型.
func newScoreModel(scores map[moderation.Category]float64, reason string) *mockModel {
	return &mockModel{
		fn: func(_ []llm.Message) string {
			parts := make([]string, 0, len(scores))
			for cat, score := range scores {
				parts = append(parts, fmt.Sprintf("%q: %v", string(cat), score))
			}
			categoriesJSON := "{" + joinStrings(parts, ", ") + "}"
			return fmt.Sprintf(`{"categories": %s, "reason": %q}`, categoriesJSON, reason)
		},
	}
}

// joinStrings 将字符串切片连接为一个字符串.
func joinStrings(parts []string, sep string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += sep
		}
		result += p
	}
	return result
}

// TestKeywordModerator_Flagged 验证关键词审核器：包含暴力关键词时应被标记.
func TestKeywordModerator_Flagged(t *testing.T) {
	rules := map[moderation.Category][]string{
		moderation.CategoryViolence: {"kill", "attack", "murder"},
		moderation.CategorySpam:     {"buy now", "click here"},
	}
	mod := moderation.NewKeywordModerator(rules)
	ctx := t.Context()

	result, err := mod.Moderate(ctx, "I will kill you")
	if err != nil {
		t.Fatalf("Moderate 失败: %v", err)
	}
	if !result.Flagged {
		t.Error("期望 Flagged=true，得到 false")
	}
	if !result.Categories[moderation.CategoryViolence] {
		t.Error("期望 violence 类别命中")
	}
	if result.Scores[moderation.CategoryViolence] != 1.0 {
		t.Errorf("期望 violence 分数=1.0，得到 %v", result.Scores[moderation.CategoryViolence])
	}
	// 未命中垃圾信息类别.
	if result.Categories[moderation.CategorySpam] {
		t.Error("期望 spam 类别未命中")
	}
	if result.Scores[moderation.CategorySpam] != 0.0 {
		t.Errorf("期望 spam 分数=0.0，得到 %v", result.Scores[moderation.CategorySpam])
	}
}

// TestKeywordModerator_Clean 验证关键词审核器：干净文本不应被标记.
func TestKeywordModerator_Clean(t *testing.T) {
	rules := map[moderation.Category][]string{
		moderation.CategoryViolence: {"kill", "attack"},
		moderation.CategoryHate:     {"hate", "racist"},
	}
	mod := moderation.NewKeywordModerator(rules)
	ctx := t.Context()

	result, err := mod.Moderate(ctx, "Today is a beautiful day")
	if err != nil {
		t.Fatalf("Moderate 失败: %v", err)
	}
	if result.Flagged {
		t.Error("期望 Flagged=false，得到 true")
	}
	for _, cat := range []moderation.Category{moderation.CategoryViolence, moderation.CategoryHate} {
		if result.Categories[cat] {
			t.Errorf("期望 %s 类别未命中", cat)
		}
		if result.Scores[cat] != 0.0 {
			t.Errorf("期望 %s 分数=0.0，得到 %v", cat, result.Scores[cat])
		}
	}
}

// TestLLMModerator 验证 LLM 审核器：解析模型返回的 JSON 分数并正确标记.
func TestLLMModerator(t *testing.T) {
	scores := map[moderation.Category]float64{
		moderation.CategoryViolence:  0.9,
		moderation.CategorySexual:    0.1,
		moderation.CategoryHate:      0.05,
		moderation.CategorySelfHarm:  0.0,
		moderation.CategoryDangerous: 0.2,
		moderation.CategoryPolitical: 0.0,
		moderation.CategorySpam:      0.0,
	}
	model := newScoreModel(scores, "包含暴力内容")
	mod := moderation.NewLLMModerator(model)
	ctx := t.Context()

	result, err := mod.Moderate(ctx, "I will hurt you badly")
	if err != nil {
		t.Fatalf("Moderate 失败: %v", err)
	}
	if !result.Flagged {
		t.Error("期望 Flagged=true，得到 false")
	}
	if !result.Categories[moderation.CategoryViolence] {
		t.Error("期望 violence 类别命中")
	}
	if result.Reason != "包含暴力内容" {
		t.Errorf("期望 Reason=包含暴力内容，得到 %q", result.Reason)
	}
	if !model.called {
		t.Error("期望模型被调用")
	}
}

// TestLLMModerator_WithThreshold 验证 LLM 审核器：分数低于阈值时不应被标记.
func TestLLMModerator_WithThreshold(t *testing.T) {
	scores := map[moderation.Category]float64{
		moderation.CategoryViolence:  0.5,
		moderation.CategorySexual:    0.3,
		moderation.CategoryHate:      0.2,
		moderation.CategorySelfHarm:  0.1,
		moderation.CategoryDangerous: 0.4,
		moderation.CategoryPolitical: 0.0,
		moderation.CategorySpam:      0.0,
	}
	model := newScoreModel(scores, "内容较为正常")
	// 阈值设为 0.8，所有分数均低于此阈值，不应被标记.
	mod := moderation.NewLLMModerator(model, moderation.WithThreshold(0.8))
	ctx := t.Context()

	result, err := mod.Moderate(ctx, "some borderline content")
	if err != nil {
		t.Fatalf("Moderate 失败: %v", err)
	}
	if result.Flagged {
		t.Error("期望 Flagged=false（分数未超过高阈值），得到 true")
	}
	for cat, score := range result.Scores {
		if result.Categories[cat] {
			t.Errorf("期望类别 %s 未命中，分数=%v", cat, score)
		}
	}
}

// TestLLMModerator_WithCategories 验证 LLM 审核器：仅检测指定类别.
func TestLLMModerator_WithCategories(t *testing.T) {
	scores := map[moderation.Category]float64{
		moderation.CategoryViolence: 0.9,
		moderation.CategorySpam:     0.8,
	}
	model := newScoreModel(scores, "包含违规内容")
	// 仅检测 spam 类别.
	mod := moderation.NewLLMModerator(
		model,
		moderation.WithCategories(moderation.CategorySpam),
	)
	ctx := t.Context()

	result, err := mod.Moderate(ctx, "buy now click here spam content")
	if err != nil {
		t.Fatalf("Moderate 失败: %v", err)
	}
	// spam 分数 0.8 > 默认阈值 0.7，应被标记.
	if !result.Flagged {
		t.Error("期望 Flagged=true（spam 超过阈值）")
	}
	if !result.Categories[moderation.CategorySpam] {
		t.Error("期望 spam 类别命中")
	}
	// violence 不在检测范围，结果中不应出现.
	if _, exists := result.Scores[moderation.CategoryViolence]; exists {
		t.Error("期望 violence 不在结果中（未指定检测）")
	}
}

// TestCompositeModerator 验证组合审核器：关键词命中后不应调用 LLM 模型.
func TestCompositeModerator(t *testing.T) {
	rules := map[moderation.Category][]string{
		moderation.CategoryViolence: {"kill", "murder"},
	}
	keywordMod := moderation.NewKeywordModerator(rules)

	// LLM 模型不应被调用.
	llmModel := &mockModel{
		fn: func(_ []llm.Message) string {
			return `{"categories": {"violence": 0.95}, "reason": "LLM 检测"}`
		},
	}
	llmMod := moderation.NewLLMModerator(llmModel)

	composite := moderation.NewCompositeModerator(keywordMod, llmMod)
	ctx := t.Context()

	result, err := composite.Moderate(ctx, "I want to kill someone")
	if err != nil {
		t.Fatalf("Moderate 失败: %v", err)
	}
	if !result.Flagged {
		t.Error("期望 Flagged=true")
	}
	if !result.Categories[moderation.CategoryViolence] {
		t.Error("期望 violence 类别命中")
	}
	// 关键词审核器已触发，LLM 不应被调用（短路优化）.
	if llmModel.called {
		t.Error("关键词审核器命中后，LLM 不应被调用")
	}
}

// TestModerateMessages 验证消息列表审核：多条消息内容拼接后审核.
func TestModerateMessages(t *testing.T) {
	rules := map[moderation.Category][]string{
		moderation.CategoryHate: {"hate", "racist"},
	}
	mod := moderation.NewKeywordModerator(rules)
	ctx := t.Context()

	messages := []llm.Message{
		llm.UserMessage("hello world"),
		llm.UserMessage("I hate everything"),
		llm.AssistantMessage("I understand"),
	}

	result, err := mod.ModerateMessages(ctx, messages)
	if err != nil {
		t.Fatalf("ModerateMessages 失败: %v", err)
	}
	if !result.Flagged {
		t.Error("期望 Flagged=true（消息中含仇恨关键词）")
	}
	if !result.Categories[moderation.CategoryHate] {
		t.Error("期望 hate 类别命中")
	}
}

// TestEmptyText 验证空文本时返回 ErrEmptyText.
func TestEmptyText(t *testing.T) {
	ctx := t.Context()

	// 关键词审核器空文本.
	t.Run("KeywordModerator", func(t *testing.T) {
		mod := moderation.NewKeywordModerator(map[moderation.Category][]string{
			moderation.CategorySpam: {"spam"},
		})
		_, err := mod.Moderate(ctx, "")
		if !errors.Is(err, moderation.ErrEmptyText) {
			t.Errorf("期望 ErrEmptyText，得到: %v", err)
		}
	})

	// 关键词审核器纯空白文本.
	t.Run("KeywordModerator_Whitespace", func(t *testing.T) {
		mod := moderation.NewKeywordModerator(map[moderation.Category][]string{
			moderation.CategorySpam: {"spam"},
		})
		_, err := mod.Moderate(ctx, "   ")
		if !errors.Is(err, moderation.ErrEmptyText) {
			t.Errorf("期望 ErrEmptyText，得到: %v", err)
		}
	})

	// LLM 审核器空文本.
	t.Run("LLMModerator", func(t *testing.T) {
		model := &mockModel{fn: func(_ []llm.Message) string { return "{}" }}
		mod := moderation.NewLLMModerator(model)
		_, err := mod.Moderate(ctx, "")
		if !errors.Is(err, moderation.ErrEmptyText) {
			t.Errorf("期望 ErrEmptyText，得到: %v", err)
		}
	})

	// 组合审核器空文本.
	t.Run("CompositeModerator", func(t *testing.T) {
		kw := moderation.NewKeywordModerator(map[moderation.Category][]string{})
		composite := moderation.NewCompositeModerator(kw)
		_, err := composite.Moderate(ctx, "")
		if !errors.Is(err, moderation.ErrEmptyText) {
			t.Errorf("期望 ErrEmptyText，得到: %v", err)
		}
	})
}
