package handoff

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Tsukikage7/servex/v2/llm"
)

// ──────────────────────────────────────────
// Mock 实现
// ──────────────────────────────────────────

// mockChatModel 测试用 mock LLM.
type mockChatModel struct {
	generateFn   func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error)
	calls        int
	lastMessages []llm.Message
}

func (m *mockChatModel) Generate(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
	m.calls++
	m.lastMessages = messages
	if m.generateFn != nil {
		return m.generateFn(ctx, messages, opts...)
	}
	return &llm.ChatResponse{Message: llm.AssistantMessage(`{"should_handoff":false,"reason":""}`)}, nil
}

func (m *mockChatModel) Stream(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (llm.StreamReader, error) {
	return nil, errors.New("mockChatModel: Stream unused")
}

// stubDetector 固定行为的 Detector，用于测试 CompositeDetector.
type stubDetector struct {
	sig *Signal
	err error
	// calls 被调用次数，用于验证短路.
	calls int
}

func (d *stubDetector) Detect(_ context.Context, _ DetectInput) (*Signal, error) {
	d.calls++
	return d.sig, d.err
}

// ──────────────────────────────────────────
// KeywordDetector
// ──────────────────────────────────────────

// TestKeywordDetector_MatchesQuestion 验证关键词命中用户问题.
func TestKeywordDetector_MatchesQuestion(t *testing.T) {
	d := NewKeywordDetector([]string{"转人工", "投诉"})
	sig, err := d.Detect(context.Background(), DetectInput{Question: "我要转人工客服"})
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
	if !sig.Should {
		t.Fatal("期望命中转人工")
	}
	if sig.Reason != ReasonKeyword {
		t.Errorf("期望 Reason=%s，实际=%s", ReasonKeyword, sig.Reason)
	}
	if sig.Meta["matched"] != "转人工" {
		t.Errorf("期望 matched=转人工，实际=%q", sig.Meta["matched"])
	}
	if sig.Meta["source"] != "question" {
		t.Errorf("期望 source=question，实际=%q", sig.Meta["source"])
	}
}

// TestKeywordDetector_MatchesAnswer 验证关键词命中 Answer 字段.
func TestKeywordDetector_MatchesAnswer(t *testing.T) {
	d := NewKeywordDetector([]string{"抱歉我无法回答"})
	sig, err := d.Detect(context.Background(), DetectInput{
		Question: "帮我查订单",
		Answer:   "抱歉我无法回答这个问题，请联系客服。",
	})
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
	if !sig.Should {
		t.Fatal("期望命中转人工")
	}
	if sig.Meta["source"] != "answer" {
		t.Errorf("期望 source=answer，实际=%q", sig.Meta["source"])
	}
}

// TestKeywordDetector_NoMatch 验证未命中时返回 Should=false.
func TestKeywordDetector_NoMatch(t *testing.T) {
	d := NewKeywordDetector([]string{"转人工"})
	sig, err := d.Detect(context.Background(), DetectInput{Question: "今天天气怎么样"})
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
	if sig.Should {
		t.Fatal("期望未命中")
	}
}

// TestKeywordDetector_EmptyKeywords 验证空关键词列表永不命中.
func TestKeywordDetector_EmptyKeywords(t *testing.T) {
	d := NewKeywordDetector(nil)
	sig, err := d.Detect(context.Background(), DetectInput{Question: "任何内容"})
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
	if sig.Should {
		t.Fatal("空关键词列表不应命中")
	}
}

// TestKeywordDetector_SkipsEmptyStrings 验证传入的空字符串关键词被忽略.
func TestKeywordDetector_SkipsEmptyStrings(t *testing.T) {
	d := NewKeywordDetector([]string{"", "转人工", ""})
	sig, err := d.Detect(context.Background(), DetectInput{Question: "你好"})
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
	if sig.Should {
		t.Fatal("空字符串关键词不应命中空问题")
	}
}

// TestKeywordDetector_QuestionPrecedesAnswer 验证 Question 优先于 Answer 匹配.
func TestKeywordDetector_QuestionPrecedesAnswer(t *testing.T) {
	d := NewKeywordDetector([]string{"投诉"})
	sig, err := d.Detect(context.Background(), DetectInput{
		Question: "我要投诉",
		Answer:   "我们记录您的投诉",
	})
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
	if sig.Meta["source"] != "question" {
		t.Errorf("期望 source=question（Question 优先），实际=%q", sig.Meta["source"])
	}
}

// ──────────────────────────────────────────
// LowConfidenceDetector
// ──────────────────────────────────────────

// TestLowConfidenceDetector_Boundary 验证 score 阈值边界行为.
func TestLowConfidenceDetector_Boundary(t *testing.T) {
	tests := []struct {
		name      string
		threshold float32
		score     float32
		want      bool
	}{
		{"score below threshold triggers", 0.5, 0.3, true},
		{"score equals threshold not trigger", 0.5, 0.5, false},
		{"score above threshold not trigger", 0.5, 0.8, false},
		{"zero score (unset) not trigger", 0.5, 0, false},
		{"negative score (unset) not trigger", 0.5, -0.1, false},
		{"threshold zero disables detector", 0, 0.1, false},
		{"threshold negative disables detector", -0.1, 0.01, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := NewLowConfidenceDetector(tc.threshold)
			sig, err := d.Detect(context.Background(), DetectInput{LastScore: tc.score})
			if err != nil {
				t.Fatalf("意外错误: %v", err)
			}
			if sig.Should != tc.want {
				t.Errorf("threshold=%v score=%v: 期望 Should=%v，实际=%v", tc.threshold, tc.score, tc.want, sig.Should)
			}
			if tc.want {
				if sig.Reason != ReasonLowConfidence {
					t.Errorf("期望 Reason=%s，实际=%s", ReasonLowConfidence, sig.Reason)
				}
				if sig.Meta["score"] == "" {
					t.Error("期望 Meta 中含 score")
				}
				if sig.Meta["threshold"] == "" {
					t.Error("期望 Meta 中含 threshold")
				}
			}
		})
	}
}

// ──────────────────────────────────────────
// RetryDetector
// ──────────────────────────────────────────

// TestRetryDetector_Boundary 验证重试次数边界.
func TestRetryDetector_Boundary(t *testing.T) {
	tests := []struct {
		name     string
		maxRetry int
		count    int
		want     bool
	}{
		{"count less than max not trigger", 3, 2, false},
		{"count equals max triggers", 3, 3, true},
		{"count exceeds max triggers", 3, 10, true},
		{"max zero disables detector", 0, 100, false},
		{"max negative disables detector", -1, 100, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := NewRetryDetector(tc.maxRetry)
			sig, err := d.Detect(context.Background(), DetectInput{RetryCount: tc.count})
			if err != nil {
				t.Fatalf("意外错误: %v", err)
			}
			if sig.Should != tc.want {
				t.Errorf("maxRetry=%d count=%d: 期望 Should=%v，实际=%v", tc.maxRetry, tc.count, tc.want, sig.Should)
			}
			if tc.want {
				if sig.Reason != ReasonRetryExceeded {
					t.Errorf("期望 Reason=%s，实际=%s", ReasonRetryExceeded, sig.Reason)
				}
				if sig.Meta["retry_count"] != fmt.Sprintf("%d", tc.count) {
					t.Errorf("Meta retry_count 错：%q", sig.Meta["retry_count"])
				}
			}
		})
	}
}

// ──────────────────────────────────────────
// CompositeDetector
// ──────────────────────────────────────────

// TestCompositeDetector_FirstMatchShortCircuits 验证命中后短路不再调用后续 Detector.
func TestCompositeDetector_FirstMatchShortCircuits(t *testing.T) {
	d1 := &stubDetector{sig: notTriggered()}
	d2 := &stubDetector{sig: &Signal{Should: true, Reason: "d2"}}
	d3 := &stubDetector{sig: &Signal{Should: true, Reason: "d3"}}

	c := NewCompositeDetector(d1, d2, d3)
	sig, err := c.Detect(context.Background(), DetectInput{})
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
	if !sig.Should || sig.Reason != "d2" {
		t.Errorf("期望 d2 命中，实际 Should=%v Reason=%s", sig.Should, sig.Reason)
	}
	if d1.calls != 1 {
		t.Errorf("d1 应被调用 1 次，实际=%d", d1.calls)
	}
	if d2.calls != 1 {
		t.Errorf("d2 应被调用 1 次，实际=%d", d2.calls)
	}
	if d3.calls != 0 {
		t.Errorf("d3 被短路不应被调用，实际=%d", d3.calls)
	}
}

// TestCompositeDetector_AllMiss 验证全部未命中时返回 Should=false.
func TestCompositeDetector_AllMiss(t *testing.T) {
	c := NewCompositeDetector(
		&stubDetector{sig: notTriggered()},
		&stubDetector{sig: notTriggered()},
	)
	sig, err := c.Detect(context.Background(), DetectInput{})
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
	if sig.Should {
		t.Fatal("期望全部未命中")
	}
}

// TestCompositeDetector_PropagatesError 验证任一子 Detector 报错立即返回 err.
func TestCompositeDetector_PropagatesError(t *testing.T) {
	sentinel := errors.New("detector boom")
	d1 := &stubDetector{sig: notTriggered()}
	d2 := &stubDetector{err: sentinel}
	d3 := &stubDetector{sig: &Signal{Should: true}}
	c := NewCompositeDetector(d1, d2, d3)
	_, err := c.Detect(context.Background(), DetectInput{})
	if !errors.Is(err, sentinel) {
		t.Fatalf("期望 sentinel，实际=%v", err)
	}
	if d3.calls != 0 {
		t.Error("d3 不应被调用")
	}
}

// TestCompositeDetector_SkipsNil 验证 nil Detector 被跳过不 panic.
func TestCompositeDetector_SkipsNil(t *testing.T) {
	d := &stubDetector{sig: &Signal{Should: true, Reason: "ok"}}
	c := NewCompositeDetector(nil, d, nil)
	sig, err := c.Detect(context.Background(), DetectInput{})
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
	if !sig.Should {
		t.Fatal("期望命中")
	}
}

// TestCompositeDetector_Empty 验证空 Detector 列表返回未命中.
func TestCompositeDetector_Empty(t *testing.T) {
	c := NewCompositeDetector()
	sig, err := c.Detect(context.Background(), DetectInput{})
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
	if sig.Should {
		t.Fatal("空列表不应命中")
	}
}

// ──────────────────────────────────────────
// LLMDetector
// ──────────────────────────────────────────

// TestNewLLMDetector_NilModel 验证 model 为 nil 时返回 ErrNilModel.
func TestNewLLMDetector_NilModel(t *testing.T) {
	d, err := NewLLMDetector(nil)
	if !errors.Is(err, ErrNilModel) {
		t.Errorf("期望 ErrNilModel，实际=%v", err)
	}
	if d != nil {
		t.Errorf("期望 nil Detector，实际=%v", d)
	}
}

// TestLLMDetector_ShouldHandoff 验证 LLM 返回 should_handoff=true 时命中.
func TestLLMDetector_ShouldHandoff(t *testing.T) {
	m := &mockChatModel{
		generateFn: func(_ context.Context, _ []llm.Message, _ ...llm.CallOption) (*llm.ChatResponse, error) {
			return &llm.ChatResponse{Message: llm.AssistantMessage(`{"should_handoff":true,"reason":"用户情绪激烈"}`)}, nil
		},
	}
	d, err := NewLLMDetector(m)
	if err != nil {
		t.Fatalf("NewLLMDetector: %v", err)
	}
	sig, err := d.Detect(context.Background(), DetectInput{Question: "你们太差了"})
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if !sig.Should {
		t.Fatal("期望命中")
	}
	if sig.Reason != ReasonLLMDetected {
		t.Errorf("期望 Reason=%s，实际=%s", ReasonLLMDetected, sig.Reason)
	}
	if sig.Meta["llm_reason"] != "用户情绪激烈" {
		t.Errorf("Meta llm_reason 错：%q", sig.Meta["llm_reason"])
	}
	if m.calls != 1 {
		t.Errorf("LLM 应被调用 1 次，实际=%d", m.calls)
	}
}

// TestLLMDetector_NotHandoff 验证 should_handoff=false 时不命中.
func TestLLMDetector_NotHandoff(t *testing.T) {
	m := &mockChatModel{
		generateFn: func(_ context.Context, _ []llm.Message, _ ...llm.CallOption) (*llm.ChatResponse, error) {
			return &llm.ChatResponse{Message: llm.AssistantMessage(`{"should_handoff":false,"reason":""}`)}, nil
		},
	}
	d, _ := NewLLMDetector(m)
	sig, err := d.Detect(context.Background(), DetectInput{Question: "你好"})
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if sig.Should {
		t.Fatal("期望未命中")
	}
}

// TestLLMDetector_MarkdownWrappedJSON 验证 LLM 输出 ```json ... ``` 也能解析.
func TestLLMDetector_MarkdownWrappedJSON(t *testing.T) {
	m := &mockChatModel{
		generateFn: func(_ context.Context, _ []llm.Message, _ ...llm.CallOption) (*llm.ChatResponse, error) {
			return &llm.ChatResponse{Message: llm.AssistantMessage("```json\n{\"should_handoff\":true,\"reason\":\"r\"}\n```")}, nil
		},
	}
	d, _ := NewLLMDetector(m)
	sig, err := d.Detect(context.Background(), DetectInput{Question: "q"})
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if !sig.Should {
		t.Fatal("期望命中（JSON 应被 ExtractJSON 剥掉 markdown 壳）")
	}
}

// TestLLMDetector_LLMError 验证 LLM 报错时返回 (未命中, err).
func TestLLMDetector_LLMError(t *testing.T) {
	sentinel := errors.New("upstream down")
	m := &mockChatModel{
		generateFn: func(_ context.Context, _ []llm.Message, _ ...llm.CallOption) (*llm.ChatResponse, error) {
			return nil, sentinel
		},
	}
	d, _ := NewLLMDetector(m)
	sig, err := d.Detect(context.Background(), DetectInput{Question: "q"})
	if err == nil {
		t.Fatal("期望 error")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("期望可以 errors.Is 匹配底层错误，实际=%v", err)
	}
	if sig == nil || sig.Should {
		t.Error("期望返回保守的未命中 Signal")
	}
}

// TestLLMDetector_ParseError 验证 LLM 输出非法 JSON 时返回错误.
func TestLLMDetector_ParseError(t *testing.T) {
	m := &mockChatModel{
		generateFn: func(_ context.Context, _ []llm.Message, _ ...llm.CallOption) (*llm.ChatResponse, error) {
			return &llm.ChatResponse{Message: llm.AssistantMessage("not a json")}, nil
		},
	}
	d, _ := NewLLMDetector(m)
	sig, err := d.Detect(context.Background(), DetectInput{Question: "q"})
	if err == nil {
		t.Fatal("期望解析错误")
	}
	if sig == nil || sig.Should {
		t.Error("期望返回保守的未命中 Signal")
	}
}

// TestLLMDetector_CustomPrompt 验证自定义 system prompt 生效.
func TestLLMDetector_CustomPrompt(t *testing.T) {
	m := &mockChatModel{
		generateFn: func(_ context.Context, messages []llm.Message, _ ...llm.CallOption) (*llm.ChatResponse, error) {
			if len(messages) == 0 || messages[0].Role != llm.RoleSystem {
				t.Fatal("期望首条为 system 消息")
			}
			if !strings.Contains(messages[0].Content, "CUSTOM") {
				t.Errorf("未应用自定义 prompt，实际=%q", messages[0].Content)
			}
			return &llm.ChatResponse{Message: llm.AssistantMessage(`{"should_handoff":false,"reason":""}`)}, nil
		},
	}
	d, err := NewLLMDetector(m, WithLLMSystemPrompt("CUSTOM PROMPT"))
	if err != nil {
		t.Fatalf("NewLLMDetector: %v", err)
	}
	_, _ = d.Detect(context.Background(), DetectInput{Question: "q"})
}

// TestLLMDetector_HistoryInUserMessage 验证 history 会拼到 user 消息中.
func TestLLMDetector_HistoryInUserMessage(t *testing.T) {
	m := &mockChatModel{
		generateFn: func(_ context.Context, messages []llm.Message, _ ...llm.CallOption) (*llm.ChatResponse, error) {
			if len(messages) < 2 {
				t.Fatal("期望至少 2 条消息")
			}
			userContent := messages[1].Content
			if !strings.Contains(userContent, "前文内容") {
				t.Errorf("user 消息应含历史，实际=%q", userContent)
			}
			if !strings.Contains(userContent, "当前问题") {
				t.Errorf("user 消息应含当前 question，实际=%q", userContent)
			}
			return &llm.ChatResponse{Message: llm.AssistantMessage(`{"should_handoff":false,"reason":""}`)}, nil
		},
	}
	d, _ := NewLLMDetector(m)
	_, _ = d.Detect(context.Background(), DetectInput{
		Question: "当前问题",
		History:  []llm.Message{llm.UserMessage("前文内容")},
	})
}

// ──────────────────────────────────────────
// FuncHook
// ──────────────────────────────────────────

// TestNewFuncHook_NilFunc 验证 nil fn 返回 ErrNilHookFunc.
func TestNewFuncHook_NilFunc(t *testing.T) {
	h, err := NewFuncHook(nil)
	if !errors.Is(err, ErrNilHookFunc) {
		t.Errorf("期望 ErrNilHookFunc，实际=%v", err)
	}
	if h != nil {
		t.Errorf("期望 nil Hook，实际=%v", h)
	}
}

// TestFuncHook_Fire 验证 Fire 转调底层函数.
func TestFuncHook_Fire(t *testing.T) {
	var (
		gotSig   *Signal
		gotInput DetectInput
	)
	sentinel := errors.New("inner err")
	h, err := NewFuncHook(func(_ context.Context, s *Signal, i DetectInput) error {
		gotSig = s
		gotInput = i
		return sentinel
	})
	if err != nil {
		t.Fatalf("NewFuncHook: %v", err)
	}
	sig := &Signal{Should: true, Reason: "r"}
	in := DetectInput{Question: "q"}
	err = h.Fire(context.Background(), sig, in)
	if !errors.Is(err, sentinel) {
		t.Errorf("期望 sentinel，实际=%v", err)
	}
	if gotSig != sig {
		t.Error("期望拿到相同 Signal")
	}
	if gotInput.Question != "q" {
		t.Errorf("期望拿到 input，实际=%+v", gotInput)
	}
}

// ──────────────────────────────────────────
// WebhookHook
// ──────────────────────────────────────────

// TestWebhookHook_PostsJSON 验证 Fire POST 正确的 JSON body.
func TestWebhookHook_PostsJSON(t *testing.T) {
	var (
		gotMethod      string
		gotContentType string
		gotAuth        string
		gotPayload     webhookPayload
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotPayload)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	h := NewWebhookHook(srv.URL, WithHeader("Authorization", "Bearer token-x"))
	sig := &Signal{
		Should: true,
		Reason: ReasonKeyword,
		Meta:   map[string]string{"matched": "转人工"},
	}
	in := DetectInput{
		Question:   "我要转人工",
		Answer:     "好的",
		RetryCount: 2,
		LastScore:  0.3,
		History:    []llm.Message{llm.UserMessage("你好")},
	}
	if err := h.Fire(context.Background(), sig, in); err != nil {
		t.Fatalf("Fire: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("期望 POST，实际=%s", gotMethod)
	}
	if gotContentType != "application/json" {
		t.Errorf("期望 Content-Type=application/json，实际=%s", gotContentType)
	}
	if gotAuth != "Bearer token-x" {
		t.Errorf("期望 Authorization=Bearer token-x，实际=%s", gotAuth)
	}
	if !gotPayload.Should || gotPayload.Reason != ReasonKeyword {
		t.Errorf("payload should/reason 不正确：%+v", gotPayload)
	}
	if gotPayload.Meta["matched"] != "转人工" {
		t.Errorf("payload meta 不正确：%+v", gotPayload.Meta)
	}
	if gotPayload.Question != "我要转人工" {
		t.Errorf("payload question 不正确：%q", gotPayload.Question)
	}
	if gotPayload.RetryCount != 2 {
		t.Errorf("payload retry_count 不正确：%d", gotPayload.RetryCount)
	}
}

// TestWebhookHook_Non2xxReturnsError 验证非 2xx 响应返回 error.
func TestWebhookHook_Non2xxReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	h := NewWebhookHook(srv.URL)
	err := h.Fire(context.Background(), &Signal{Should: true}, DetectInput{})
	if err == nil {
		t.Fatal("期望返回错误")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("期望错误含状态码 500，实际=%v", err)
	}
}

// TestWebhookHook_NetworkError 验证网络错误返回 error.
func TestWebhookHook_NetworkError(t *testing.T) {
	// 关闭的 server，确保连接被拒绝.
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	srv.Close()

	h := NewWebhookHook(srv.URL)
	err := h.Fire(context.Background(), &Signal{Should: true}, DetectInput{})
	if err == nil {
		t.Fatal("期望网络错误")
	}
}

// TestWebhookHook_NilSignal 验证传入 nil signal 返回错误.
func TestWebhookHook_NilSignal(t *testing.T) {
	h := NewWebhookHook("http://localhost")
	err := h.Fire(context.Background(), nil, DetectInput{})
	if err == nil {
		t.Fatal("期望 nil signal 报错")
	}
}

// TestWebhookHook_CustomClient 验证 WithHTTPClient 覆盖默认 client.
func TestWebhookHook_CustomClient(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	custom := &http.Client{Timeout: 2 * time.Second}
	h := NewWebhookHook(srv.URL, WithHTTPClient(custom))
	if err := h.Fire(context.Background(), &Signal{Should: true}, DetectInput{}); err != nil {
		t.Fatalf("Fire: %v", err)
	}
	if !called {
		t.Error("期望 server 被调用")
	}
}

// TestWebhookHook_ContextCancellation 验证 ctx 取消会中断请求.
func TestWebhookHook_ContextCancellation(t *testing.T) {
	// 让 server 阻塞足够久.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h := NewWebhookHook(srv.URL, WithTimeout(10*time.Second))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	err := h.Fire(ctx, &Signal{Should: true}, DetectInput{})
	if err == nil {
		t.Fatal("期望 ctx 取消后返回错误")
	}
}

// ──────────────────────────────────────────
// 常量与辅助
// ──────────────────────────────────────────

// TestReasonConstants 记录并验证 Reason 常量不会漂移（纯 documentation 用途）.
func TestReasonConstants(t *testing.T) {
	if ReasonKeyword != "keyword" || ReasonLowConfidence != "low_confidence" ||
		ReasonRetryExceeded != "retry_exceeded" || ReasonLLMDetected != "llm_detected" {
		t.Error("Reason 常量值不应随意修改（会破坏下游按字符串分发）")
	}
}
