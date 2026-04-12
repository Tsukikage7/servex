package extractor_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Tsukikage7/servex/llm"
	"github.com/Tsukikage7/servex/llm/processing/extractor"
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

// TestEntityExtractor 验证实体识别提取器：正确识别人名和组织.
func TestEntityExtractor(t *testing.T) {
	model := &mockModel{
		fn: func(_ []llm.Message) string {
			return `[{"text":"张伟","type":"person","start":0,"end":2,"metadata":{}},{"text":"阿里巴巴","type":"organization","start":5,"end":9,"metadata":{}}]`
		},
	}
	e := extractor.NewEntityExtractor(model, []string{"person", "organization"})
	ctx := t.Context()

	result, err := e.Extract(ctx, "张伟加入了阿里巴巴集团")
	if err != nil {
		t.Fatalf("Extract 失败: %v", err)
	}
	if len(result.Entities) != 2 {
		t.Errorf("期望 2 个实体，得到 %d", len(result.Entities))
	}
	if result.Entities[0].Text != "张伟" {
		t.Errorf("期望第一个实体为 张伟，得到 %q", result.Entities[0].Text)
	}
	if result.Entities[0].Type != "person" {
		t.Errorf("期望第一个实体类型为 person，得到 %q", result.Entities[0].Type)
	}
	if result.Entities[1].Text != "阿里巴巴" {
		t.Errorf("期望第二个实体为 阿里巴巴，得到 %q", result.Entities[1].Text)
	}
	if result.Entities[1].Type != "organization" {
		t.Errorf("期望第二个实体类型为 organization，得到 %q", result.Entities[1].Type)
	}
}

// TestRelationExtractor 验证关系抽取提取器：正确提取三元组关系.
func TestRelationExtractor(t *testing.T) {
	model := &mockModel{
		fn: func(_ []llm.Message) string {
			return `[{"subject":"李明","predicate":"创立","object":"科技公司"},{"subject":"科技公司","predicate":"位于","object":"北京"}]`
		},
	}
	e := extractor.NewRelationExtractor(model)
	ctx := t.Context()

	result, err := e.Extract(ctx, "李明在北京创立了一家科技公司")
	if err != nil {
		t.Fatalf("Extract 失败: %v", err)
	}
	if len(result.Relations) != 2 {
		t.Errorf("期望 2 个关系，得到 %d", len(result.Relations))
	}
	if result.Relations[0].Subject != "李明" {
		t.Errorf("期望第一个关系主语为 李明，得到 %q", result.Relations[0].Subject)
	}
	if result.Relations[0].Predicate != "创立" {
		t.Errorf("期望第一个关系谓词为 创立，得到 %q", result.Relations[0].Predicate)
	}
	if result.Relations[0].Object != "科技公司" {
		t.Errorf("期望第一个关系宾语为 科技公司，得到 %q", result.Relations[0].Object)
	}
}

// TestKeywordExtractor 验证关键词提取器：关键词按分数降序排列.
func TestKeywordExtractor(t *testing.T) {
	model := &mockModel{
		fn: func(_ []llm.Message) string {
			return `[{"word":"人工智能","score":0.95},{"word":"机器学习","score":0.87},{"word":"深度学习","score":0.76},{"word":"神经网络","score":0.65}]`
		},
	}
	e := extractor.NewKeywordExtractor(model)
	ctx := t.Context()

	result, err := e.Extract(ctx, "人工智能和机器学习是深度学习领域的核心技术，神经网络是基础")
	if err != nil {
		t.Fatalf("Extract 失败: %v", err)
	}
	if len(result.Keywords) != 4 {
		t.Errorf("期望 4 个关键词，得到 %d", len(result.Keywords))
	}
	if result.Keywords[0].Word != "人工智能" {
		t.Errorf("期望第一个关键词为 人工智能，得到 %q", result.Keywords[0].Word)
	}
	if result.Keywords[0].Score != 0.95 {
		t.Errorf("期望第一个关键词分数为 0.95，得到 %v", result.Keywords[0].Score)
	}
	// 验证降序排列.
	for i := 1; i < len(result.Keywords); i++ {
		if result.Keywords[i].Score > result.Keywords[i-1].Score {
			t.Errorf("关键词未按分数降序排列：索引 %d (%v) > 索引 %d (%v)", i, result.Keywords[i].Score, i-1, result.Keywords[i-1].Score)
		}
	}
}

// TestSummarizer 验证文本摘要提取器：正确生成摘要.
func TestSummarizer(t *testing.T) {
	model := &mockModel{
		fn: func(_ []llm.Message) string {
			return `{"text":"人工智能正在快速发展并应用于各个领域。","sentences":1}`
		},
	}
	e := extractor.NewSummarizer(model)
	ctx := t.Context()

	result, err := e.Extract(ctx, "人工智能技术近年来取得了突破性进展，在医疗、教育、金融等多个行业得到广泛应用，极大地提升了工作效率和服务质量。")
	if err != nil {
		t.Fatalf("Extract 失败: %v", err)
	}
	if result.Summary == nil {
		t.Fatal("期望 Summary 不为 nil")
	}
	if result.Summary.Text == "" {
		t.Error("期望 Summary.Text 不为空")
	}
	if result.Summary.Sentences != 1 {
		t.Errorf("期望 Summary.Sentences=1，得到 %d", result.Summary.Sentences)
	}
}

// TestWithMaxKeywords 验证 WithMaxKeywords 选项：截断至指定最大数量.
func TestWithMaxKeywords(t *testing.T) {
	model := &mockModel{
		fn: func(_ []llm.Message) string {
			return `[{"word":"a","score":0.9},{"word":"b","score":0.8},{"word":"c","score":0.7},{"word":"d","score":0.6},{"word":"e","score":0.5}]`
		},
	}
	e := extractor.NewKeywordExtractor(model, extractor.WithMaxKeywords(3))
	ctx := t.Context()

	result, err := e.Extract(ctx, "some text with keywords")
	if err != nil {
		t.Fatalf("Extract 失败: %v", err)
	}
	if len(result.Keywords) != 3 {
		t.Errorf("期望 3 个关键词（WithMaxKeywords=3），得到 %d", len(result.Keywords))
	}
}

// TestEmptyText 验证空文本时返回 ErrEmptyText.
func TestEmptyText(t *testing.T) {
	model := &mockModel{fn: func(_ []llm.Message) string { return "[]" }}
	ctx := t.Context()

	t.Run("EntityExtractor", func(t *testing.T) {
		e := extractor.NewEntityExtractor(model, []string{"person"})
		_, err := e.Extract(ctx, "")
		if !errors.Is(err, extractor.ErrEmptyText) {
			t.Errorf("期望 ErrEmptyText，得到: %v", err)
		}
	})

	t.Run("RelationExtractor", func(t *testing.T) {
		e := extractor.NewRelationExtractor(model)
		_, err := e.Extract(ctx, "   ")
		if !errors.Is(err, extractor.ErrEmptyText) {
			t.Errorf("期望 ErrEmptyText，得到: %v", err)
		}
	})

	t.Run("KeywordExtractor", func(t *testing.T) {
		e := extractor.NewKeywordExtractor(model)
		_, err := e.Extract(ctx, "")
		if !errors.Is(err, extractor.ErrEmptyText) {
			t.Errorf("期望 ErrEmptyText，得到: %v", err)
		}
	})

	t.Run("Summarizer", func(t *testing.T) {
		e := extractor.NewSummarizer(model)
		_, err := e.Extract(ctx, "")
		if !errors.Is(err, extractor.ErrEmptyText) {
			t.Errorf("期望 ErrEmptyText，得到: %v", err)
		}
	})
}
