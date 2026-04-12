package translator_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Tsukikage7/servex/llm"
	"github.com/Tsukikage7/servex/llm/processing/translator"
)

// mockModel 测试用模拟模型.
type mockModel struct {
	// fn 根据输入消息返回响应文本.
	fn func(msgs []llm.Message) string
	// lastMessages 记录最后一次调用的消息，用于验证提示内容.
	lastMessages []llm.Message
}

// Generate 调用 fn 生成响应，并记录消息.
func (m *mockModel) Generate(_ context.Context, msgs []llm.Message, _ ...llm.CallOption) (*llm.ChatResponse, error) {
	m.lastMessages = msgs
	content := m.fn(msgs)
	return &llm.ChatResponse{Message: llm.AssistantMessage(content)}, nil
}

// Stream 未实现，仅满足接口要求.
func (m *mockModel) Stream(_ context.Context, _ []llm.Message, _ ...llm.CallOption) (llm.StreamReader, error) {
	return nil, fmt.Errorf("not implemented")
}

// TestTranslate 验证基本翻译功能：返回正确的翻译结果和语言代码.
func TestTranslate(t *testing.T) {
	model := &mockModel{
		fn: func(_ []llm.Message) string {
			return `{"text":"Hello, World!","source_language":"zh","target_language":"en"}`
		},
	}
	tr := translator.NewTranslator(model)
	ctx := t.Context()

	result, err := tr.Translate(ctx, "你好，世界！", "en")
	if err != nil {
		t.Fatalf("Translate 失败: %v", err)
	}
	if result.Text != "Hello, World!" {
		t.Errorf("期望 Text=Hello, World!，得到 %q", result.Text)
	}
	if result.SourceLanguage != "zh" {
		t.Errorf("期望 SourceLanguage=zh，得到 %q", result.SourceLanguage)
	}
	if result.TargetLanguage != "en" {
		t.Errorf("期望 TargetLanguage=en，得到 %q", result.TargetLanguage)
	}
}

// TestTranslate_WithGlossary 验证术语表选项：系统提示应包含术语表内容.
func TestTranslate_WithGlossary(t *testing.T) {
	model := &mockModel{
		fn: func(_ []llm.Message) string {
			return `{"text":"Please submit the PR.","source_language":"zh","target_language":"en"}`
		},
	}
	glossary := map[string]string{
		"PR": "Pull Request",
	}
	tr := translator.NewTranslator(model, translator.WithGlossary(glossary))
	ctx := t.Context()

	_, err := tr.Translate(ctx, "请提交PR", "en")
	if err != nil {
		t.Fatalf("Translate 失败: %v", err)
	}

	// 验证系统消息中包含术语表.
	if len(model.lastMessages) == 0 {
		t.Fatal("期望有消息记录")
	}
	sysMsg := model.lastMessages[0].Content
	if !strings.Contains(sysMsg, "PR") || !strings.Contains(sysMsg, "Pull Request") {
		t.Errorf("期望系统提示包含术语表 PR→Pull Request，实际: %q", sysMsg)
	}
}

// TestTranslateBatch 验证批量翻译功能：3 条文本均被翻译.
func TestTranslateBatch(t *testing.T) {
	model := &mockModel{
		fn: func(_ []llm.Message) string {
			return `[{"text":"Hello","source_language":"zh","target_language":"en"},{"text":"Goodbye","source_language":"zh","target_language":"en"},{"text":"Thank you","source_language":"zh","target_language":"en"}]`
		},
	}
	tr := translator.NewTranslator(model)
	ctx := t.Context()

	texts := []string{"你好", "再见", "谢谢"}
	result, err := tr.TranslateBatch(ctx, texts, "en")
	if err != nil {
		t.Fatalf("TranslateBatch 失败: %v", err)
	}
	if len(result.Translations) != 3 {
		t.Errorf("期望 3 条翻译结果，得到 %d", len(result.Translations))
	}
	if result.Translations[0].Text != "Hello" {
		t.Errorf("期望第一条翻译为 Hello，得到 %q", result.Translations[0].Text)
	}
	if result.Translations[1].Text != "Goodbye" {
		t.Errorf("期望第二条翻译为 Goodbye，得到 %q", result.Translations[1].Text)
	}
}

// TestDetectLanguage 验证语言检测功能：正确返回语言代码.
func TestDetectLanguage(t *testing.T) {
	model := &mockModel{
		fn: func(_ []llm.Message) string {
			return "ja"
		},
	}
	tr := translator.NewTranslator(model)
	ctx := t.Context()

	lang, err := tr.DetectLanguage(ctx, "こんにちは")
	if err != nil {
		t.Fatalf("DetectLanguage 失败: %v", err)
	}
	if lang != "ja" {
		t.Errorf("期望语言代码 ja，得到 %q", lang)
	}
}

// TestEmptyText 验证空文本时返回 ErrEmptyText.
func TestEmptyText(t *testing.T) {
	model := &mockModel{fn: func(_ []llm.Message) string { return "{}" }}
	tr := translator.NewTranslator(model)
	ctx := t.Context()

	t.Run("Translate", func(t *testing.T) {
		_, err := tr.Translate(ctx, "", "en")
		if !errors.Is(err, translator.ErrEmptyText) {
			t.Errorf("期望 ErrEmptyText，得到: %v", err)
		}
	})

	t.Run("Translate_Whitespace", func(t *testing.T) {
		_, err := tr.Translate(ctx, "   ", "en")
		if !errors.Is(err, translator.ErrEmptyText) {
			t.Errorf("期望 ErrEmptyText，得到: %v", err)
		}
	})

	t.Run("DetectLanguage", func(t *testing.T) {
		_, err := tr.DetectLanguage(ctx, "")
		if !errors.Is(err, translator.ErrEmptyText) {
			t.Errorf("期望 ErrEmptyText，得到: %v", err)
		}
	})
}

// TestEmptyTarget 验证目标语言为空时返回 ErrEmptyTarget.
func TestEmptyTarget(t *testing.T) {
	model := &mockModel{fn: func(_ []llm.Message) string { return "{}" }}
	tr := translator.NewTranslator(model)
	ctx := t.Context()

	t.Run("Translate", func(t *testing.T) {
		_, err := tr.Translate(ctx, "hello", "")
		if !errors.Is(err, translator.ErrEmptyTarget) {
			t.Errorf("期望 ErrEmptyTarget，得到: %v", err)
		}
	})

	t.Run("TranslateBatch", func(t *testing.T) {
		_, err := tr.TranslateBatch(ctx, []string{"hello"}, "")
		if !errors.Is(err, translator.ErrEmptyTarget) {
			t.Errorf("期望 ErrEmptyTarget，得到: %v", err)
		}
	})
}
