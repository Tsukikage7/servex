package classifier_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/processing/classifier"
)

// mockModel 测试用模拟模型.
type mockModel struct {
	// fn 根据输入消息返回响应文本.
	fn func(msgs []llm.Message) string
}

// Generate 调用 fn 生成响应.
func (m *mockModel) Generate(_ context.Context, msgs []llm.Message, _ ...llm.CallOption) (*llm.ChatResponse, error) {
	content := m.fn(msgs)
	return &llm.ChatResponse{Message: llm.AssistantMessage(content)}, nil
}

// Stream 未实现，仅满足接口要求.
func (m *mockModel) Stream(_ context.Context, _ []llm.Message, _ ...llm.CallOption) (llm.StreamReader, error) {
	return nil, fmt.Errorf("not implemented")
}

// TestIntentClassifier 验证意图识别分类器：返回正确的意图及最高分.
func TestIntentClassifier(t *testing.T) {
	model := &mockModel{
		fn: func(_ []llm.Message) string {
			return `[{"name":"buy","score":0.9,"description":"用户想购买"},{"name":"refund","score":0.1,"description":""}]`
		},
	}
	intents := map[string]string{
		"buy":    "用户想购买商品",
		"refund": "用户想申请退款",
	}
	c := classifier.NewIntentClassifier(model, intents)
	ctx := t.Context()

	result, err := c.Classify(ctx, "我想买一台手机")
	if err != nil {
		t.Fatalf("Classify 失败: %v", err)
	}
	if result.Best.Name != "buy" {
		t.Errorf("期望 Best.Name=buy，得到 %q", result.Best.Name)
	}
	if result.Best.Score != 0.9 {
		t.Errorf("期望 Best.Score=0.9，得到 %v", result.Best.Score)
	}
	if len(result.Labels) != 2 {
		t.Errorf("期望 2 个标签，得到 %d", len(result.Labels))
	}
	// 验证按分数降序.
	if result.Labels[0].Score < result.Labels[1].Score {
		t.Error("期望标签按分数降序排列")
	}
}

// TestSentimentClassifier 验证情感分析分类器：正面文本应返回 positive 为最高分.
func TestSentimentClassifier(t *testing.T) {
	model := &mockModel{
		fn: func(_ []llm.Message) string {
			return `[{"name":"positive","score":0.92,"description":"积极表达"},{"name":"neutral","score":0.06,"description":""},{"name":"negative","score":0.02,"description":""}]`
		},
	}
	c := classifier.NewSentimentClassifier(model)
	ctx := t.Context()

	result, err := c.Classify(ctx, "今天天气真棒，心情超好！")
	if err != nil {
		t.Fatalf("Classify 失败: %v", err)
	}
	if result.Best.Name != "positive" {
		t.Errorf("期望 Best.Name=positive，得到 %q", result.Best.Name)
	}
	if result.Best.Score != 0.92 {
		t.Errorf("期望 Best.Score=0.92，得到 %v", result.Best.Score)
	}
}

// TestTopicClassifier 验证主题分类器：从给定主题中选择最匹配的.
func TestTopicClassifier(t *testing.T) {
	model := &mockModel{
		fn: func(_ []llm.Message) string {
			return `[{"name":"technology","score":0.88,"description":"讨论科技"},{"name":"sports","score":0.07,"description":""},{"name":"finance","score":0.05,"description":""}]`
		},
	}
	topics := []string{"technology", "sports", "finance"}
	c := classifier.NewTopicClassifier(model, topics)
	ctx := t.Context()

	result, err := c.Classify(ctx, "人工智能正在改变软件开发方式")
	if err != nil {
		t.Fatalf("Classify 失败: %v", err)
	}
	if result.Best.Name != "technology" {
		t.Errorf("期望 Best.Name=technology，得到 %q", result.Best.Name)
	}
}

// TestLanguageClassifier 验证语言检测分类器：正确识别语言代码.
func TestLanguageClassifier(t *testing.T) {
	model := &mockModel{
		fn: func(_ []llm.Message) string {
			return `[{"name":"zh","score":0.97,"description":"简体中文"},{"name":"en","score":0.02,"description":"英语"}]`
		},
	}
	c := classifier.NewLanguageClassifier(model)
	ctx := t.Context()

	result, err := c.Classify(ctx, "你好世界")
	if err != nil {
		t.Fatalf("Classify 失败: %v", err)
	}
	if result.Best.Name != "zh" {
		t.Errorf("期望 Best.Name=zh，得到 %q", result.Best.Name)
	}
}

// TestRouterClassifier 验证路由分类器：选择最匹配的路由.
func TestRouterClassifier(t *testing.T) {
	model := &mockModel{
		fn: func(_ []llm.Message) string {
			return `[{"name":"weather_agent","score":0.93,"description":"天气查询请求"},{"name":"calculator","score":0.04,"description":""}]`
		},
	}
	routes := map[string]string{
		"weather_agent": "查询天气信息",
		"calculator":    "执行数学计算",
	}
	c := classifier.NewRouterClassifier(model, routes)
	ctx := t.Context()

	result, err := c.Classify(ctx, "北京今天天气怎么样？")
	if err != nil {
		t.Fatalf("Classify 失败: %v", err)
	}
	if result.Best.Name != "weather_agent" {
		t.Errorf("期望 Best.Name=weather_agent，得到 %q", result.Best.Name)
	}
}

// TestCustomClassifier 验证自定义分类器：使用自定义标签和系统提示.
func TestCustomClassifier(t *testing.T) {
	model := &mockModel{
		fn: func(_ []llm.Message) string {
			return `[{"name":"urgent","score":0.85,"description":"紧急问题"},{"name":"normal","score":0.1,"description":""},{"name":"low","score":0.05,"description":""}]`
		},
	}
	labels := []string{"urgent", "normal", "low"}
	c := classifier.NewCustomClassifier(model, labels, "根据工单内容判断优先级")
	ctx := t.Context()

	result, err := c.Classify(ctx, "系统崩溃了，无法登录！")
	if err != nil {
		t.Fatalf("Classify 失败: %v", err)
	}
	if result.Best.Name != "urgent" {
		t.Errorf("期望 Best.Name=urgent，得到 %q", result.Best.Name)
	}
}

// TestClassifyMessages 验证消息列表分类：多条消息内容拼接后分类.
func TestClassifyMessages(t *testing.T) {
	model := &mockModel{
		fn: func(_ []llm.Message) string {
			return `[{"name":"complaint","score":0.88,"description":"投诉"},{"name":"inquiry","score":0.12,"description":""}]`
		},
	}
	intents := map[string]string{
		"complaint": "投诉问题",
		"inquiry":   "一般咨询",
	}
	c := classifier.NewIntentClassifier(model, intents)
	ctx := t.Context()

	messages := []llm.Message{
		llm.UserMessage("你们的服务太差了"),
		llm.AssistantMessage("很抱歉给您带来不便"),
		llm.UserMessage("我要投诉！"),
	}

	result, err := c.ClassifyMessages(ctx, messages)
	if err != nil {
		t.Fatalf("ClassifyMessages 失败: %v", err)
	}
	if result.Best.Name != "complaint" {
		t.Errorf("期望 Best.Name=complaint，得到 %q", result.Best.Name)
	}
}

// TestEmptyText 验证空文本时返回 ErrEmptyText.
func TestEmptyText(t *testing.T) {
	model := &mockModel{fn: func(_ []llm.Message) string { return "[]" }}
	c := classifier.NewSentimentClassifier(model)
	ctx := t.Context()

	_, err := c.Classify(ctx, "")
	if !errors.Is(err, classifier.ErrEmptyText) {
		t.Errorf("期望 ErrEmptyText，得到: %v", err)
	}

	_, err = c.Classify(ctx, "   ")
	if !errors.Is(err, classifier.ErrEmptyText) {
		t.Errorf("期望 ErrEmptyText（空白字符），得到: %v", err)
	}
}

// TestWithTopN 验证 WithTopN 选项：仅返回前 N 个标签.
func TestWithTopN(t *testing.T) {
	model := &mockModel{
		fn: func(_ []llm.Message) string {
			return `[{"name":"a","score":0.9},{"name":"b","score":0.7},{"name":"c","score":0.5},{"name":"d","score":0.3}]`
		},
	}
	c := classifier.NewSentimentClassifier(model, classifier.WithTopN(2))
	ctx := t.Context()

	result, err := c.Classify(ctx, "some text")
	if err != nil {
		t.Fatalf("Classify 失败: %v", err)
	}
	if len(result.Labels) != 2 {
		t.Errorf("期望 2 个标签，得到 %d", len(result.Labels))
	}
	if result.Labels[0].Name != "a" {
		t.Errorf("期望第一个标签为 a，得到 %q", result.Labels[0].Name)
	}
	if result.Labels[1].Name != "b" {
		t.Errorf("期望第二个标签为 b，得到 %q", result.Labels[1].Name)
	}
}

// TestNoLabels 验证空标签时返回 ErrNoLabels.
func TestNoLabels(t *testing.T) {
	model := &mockModel{fn: func(_ []llm.Message) string { return "[]" }}
	ctx := t.Context()

	t.Run("IntentClassifier", func(t *testing.T) {
		c := classifier.NewIntentClassifier(model, map[string]string{})
		_, err := c.Classify(ctx, "test")
		if !errors.Is(err, classifier.ErrNoLabels) {
			t.Errorf("期望 ErrNoLabels，得到: %v", err)
		}
	})

	t.Run("RouterClassifier", func(t *testing.T) {
		c := classifier.NewRouterClassifier(model, map[string]string{})
		_, err := c.Classify(ctx, "test")
		if !errors.Is(err, classifier.ErrNoLabels) {
			t.Errorf("期望 ErrNoLabels，得到: %v", err)
		}
	})

	t.Run("CustomClassifier", func(t *testing.T) {
		c := classifier.NewCustomClassifier(model, []string{}, "prompt")
		_, err := c.Classify(ctx, "test")
		if !errors.Is(err, classifier.ErrNoLabels) {
			t.Errorf("期望 ErrNoLabels，得到: %v", err)
		}
	})
}
