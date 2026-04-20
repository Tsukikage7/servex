package rewrite

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/Tsukikage7/servex/v2/llm"
)

// ──────────────────────────────────────────
// Mock 实现（参考 rag/rag_test.go 中的 mockChatModel）
// ──────────────────────────────────────────

// mockChatModel 模拟聊天模型.
type mockChatModel struct {
	// generateFn 可选的自定义 Generate 行为.
	generateFn func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error)
	// calls Generate 被调用的次数.
	calls int
	// lastMessages 最近一次 Generate 收到的消息.
	lastMessages []llm.Message
	// lastOpts 最近一次 Generate 收到的 CallOption.
	lastOpts []llm.CallOption
}

func (m *mockChatModel) Generate(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
	m.calls++
	m.lastMessages = messages
	m.lastOpts = opts
	if m.generateFn != nil {
		return m.generateFn(ctx, messages, opts...)
	}
	return &llm.ChatResponse{
		Message: llm.AssistantMessage("mock answer"),
	}, nil
}

// Stream 本包未使用；返回固定错误以便误用时被立即发现.
func (m *mockChatModel) Stream(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (llm.StreamReader, error) {
	return nil, errors.New("mockChatModel: Stream unused")
}

// firstPromptText 提取 mock 收到的第一条消息的文本内容.
func firstPromptText(t *testing.T, m *mockChatModel) string {
	t.Helper()
	if len(m.lastMessages) == 0 {
		t.Fatalf("期望 mock 至少收到一条消息，实际为 0")
	}
	return m.lastMessages[0].Content
}

// mustHistoryAware 构造 HistoryAware 改写器，出错立即 fail.
func mustHistoryAware(t *testing.T, m llm.ChatModel, opts ...Option) Rewriter {
	t.Helper()
	r, err := NewHistoryAwareRewriter(m, opts...)
	if err != nil {
		t.Fatalf("NewHistoryAwareRewriter 失败: %v", err)
	}
	return r
}

// mustHyDE 构造 HyDE 改写器，出错立即 fail.
func mustHyDE(t *testing.T, m llm.ChatModel, opts ...Option) Rewriter {
	t.Helper()
	r, err := NewHyDERewriter(m, opts...)
	if err != nil {
		t.Fatalf("NewHyDERewriter 失败: %v", err)
	}
	return r
}

// ──────────────────────────────────────────
// 构造校验
// ──────────────────────────────────────────

// TestNew_NilModel 验证两个构造函数在 model 为 nil 时返回 ErrNilModel.
func TestNew_NilModel(t *testing.T) {
	t.Run("history-aware", func(t *testing.T) {
		r, err := NewHistoryAwareRewriter(nil)
		if !errors.Is(err, ErrNilModel) {
			t.Errorf("期望 ErrNilModel，实际=%v", err)
		}
		if r != nil {
			t.Errorf("期望返回 nil Rewriter，实际=%v", r)
		}
	})
	t.Run("hyde", func(t *testing.T) {
		r, err := NewHyDERewriter(nil)
		if !errors.Is(err, ErrNilModel) {
			t.Errorf("期望 ErrNilModel，实际=%v", err)
		}
		if r != nil {
			t.Errorf("期望返回 nil Rewriter，实际=%v", r)
		}
	})
}

// ──────────────────────────────────────────
// HistoryAwareRewriter
// ──────────────────────────────────────────

// TestHistoryAwareRewriter_EmptyHistory 验证 history 为空/nil 时不调 LLM 直接返回原 query.
func TestHistoryAwareRewriter_EmptyHistory(t *testing.T) {
	cases := []struct {
		name    string
		history []llm.Message
	}{
		{"nil history", nil},
		{"empty history", []llm.Message{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := &mockChatModel{}
			r := mustHistoryAware(t, m)
			got, err := r.Rewrite(context.Background(), "原始问题", tc.history)
			if err != nil {
				t.Fatalf("期望无错误，实际得到: %v", err)
			}
			if got != "原始问题" {
				t.Errorf("期望返回原 query，实际=%q", got)
			}
			if m.calls != 0 {
				t.Errorf("期望不调用 LLM，实际调用 %d 次", m.calls)
			}
		})
	}
}

// TestHistoryAwareRewriter_EmptyQuery 验证 query 为 "" 时不调 LLM 直接返回 "".
func TestHistoryAwareRewriter_EmptyQuery(t *testing.T) {
	m := &mockChatModel{}
	r := mustHistoryAware(t, m)
	got, err := r.Rewrite(context.Background(), "", []llm.Message{llm.UserMessage("前文")})
	if err != nil {
		t.Fatalf("期望无错误，实际得到: %v", err)
	}
	if got != "" {
		t.Errorf("期望返回空字符串，实际=%q", got)
	}
	if m.calls != 0 {
		t.Errorf("期望不调用 LLM，实际调用 %d 次", m.calls)
	}
}

// TestHistoryAwareRewriter_RewritesWithHistory 验证基于 history 调用 LLM 完成改写，
// 且 prompt 中应同时包含 history 内容和当前 query.
func TestHistoryAwareRewriter_RewritesWithHistory(t *testing.T) {
	m := &mockChatModel{
		generateFn: func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
			return &llm.ChatResponse{Message: llm.AssistantMessage("VPS 产品怎么退款")}, nil
		},
	}
	r := mustHistoryAware(t, m)

	history := []llm.Message{
		llm.UserMessage("A 产品怎么退款"),
		llm.AssistantMessage("7 天内无理由"),
	}
	got, err := r.Rewrite(context.Background(), "那 B 呢", history)
	if err != nil {
		t.Fatalf("Rewrite 失败: %v", err)
	}
	if got != "VPS 产品怎么退款" {
		t.Errorf("期望改写结果='VPS 产品怎么退款'，实际=%q", got)
	}

	prompt := firstPromptText(t, m)
	// 应含历史内容.
	if !strings.Contains(prompt, "A 产品怎么退款") {
		t.Errorf("prompt 应包含历史 user 内容，实际=%q", prompt)
	}
	if !strings.Contains(prompt, "7 天内无理由") {
		t.Errorf("prompt 应包含历史 assistant 内容，实际=%q", prompt)
	}
	// 应含当前 query.
	if !strings.Contains(prompt, "那 B 呢") {
		t.Errorf("prompt 应包含当前 query，实际=%q", prompt)
	}
	// 角色中文映射应生效.
	if !strings.Contains(prompt, "用户: A 产品怎么退款") {
		t.Errorf("prompt 应包含中文角色标签 '用户: '，实际=%q", prompt)
	}
	if !strings.Contains(prompt, "助手: 7 天内无理由") {
		t.Errorf("prompt 应包含中文角色标签 '助手: '，实际=%q", prompt)
	}
}

// TestHistoryAwareRewriter_EmptyResponseReturnsOriginal 验证 LLM 返回空（含仅空白）时回退原 query.
func TestHistoryAwareRewriter_EmptyResponseReturnsOriginal(t *testing.T) {
	cases := []struct {
		name string
		resp string
	}{
		{"empty string", ""},
		{"whitespace only", "  \n\t  "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := &mockChatModel{
				generateFn: func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
					return &llm.ChatResponse{Message: llm.AssistantMessage(tc.resp)}, nil
				},
			}
			r := mustHistoryAware(t, m)
			got, err := r.Rewrite(context.Background(), "原始 query", []llm.Message{llm.UserMessage("前文")})
			if err != nil {
				t.Fatalf("期望无错误，实际得到: %v", err)
			}
			if got != "原始 query" {
				t.Errorf("期望返回原 query，实际=%q", got)
			}
		})
	}
}

// TestHistoryAwareRewriter_LLMErrorReturnsOriginalAndErr 验证 LLM 失败时返回原 query + 非 nil error.
func TestHistoryAwareRewriter_LLMErrorReturnsOriginalAndErr(t *testing.T) {
	sentinel := errors.New("generate failed")
	m := &mockChatModel{
		generateFn: func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
			return nil, sentinel
		},
	}
	r := mustHistoryAware(t, m)
	got, err := r.Rewrite(context.Background(), "原始 query", []llm.Message{llm.UserMessage("前文")})
	if err == nil {
		t.Fatal("期望返回非 nil error")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("期望 error 可被 errors.Is 匹配到底层错误，实际=%v", err)
	}
	if got != "原始 query" {
		t.Errorf("期望返回原 query，实际=%q", got)
	}
}

// TestHistoryAwareRewriter_TrimsWhitespace 验证 LLM 输出会被 trim 两端空白.
func TestHistoryAwareRewriter_TrimsWhitespace(t *testing.T) {
	m := &mockChatModel{
		generateFn: func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
			return &llm.ChatResponse{Message: llm.AssistantMessage("  回答  \n")}, nil
		},
	}
	r := mustHistoryAware(t, m)
	got, err := r.Rewrite(context.Background(), "原始", []llm.Message{llm.UserMessage("前文")})
	if err != nil {
		t.Fatalf("Rewrite 失败: %v", err)
	}
	if got != "回答" {
		t.Errorf("期望 trim 后为 '回答'，实际=%q", got)
	}
}

// TestHistoryAwareRewriter_CustomSystemPrompt 验证自定义 system prompt 生效，
// 占位 {{.Query}}、{{.History}} 被正确替换.
func TestHistoryAwareRewriter_CustomSystemPrompt(t *testing.T) {
	m := &mockChatModel{
		generateFn: func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
			return &llm.ChatResponse{Message: llm.AssistantMessage("ok")}, nil
		},
	}
	// 模板仅含 {{.Query}}，验证即便不含 {{.History}} 也能工作.
	r := mustHistoryAware(t, m, WithSystemPrompt("自定义{{.Query}}END"))
	_, err := r.Rewrite(context.Background(), "X", []llm.Message{llm.UserMessage("前文")})
	if err != nil {
		t.Fatalf("Rewrite 失败: %v", err)
	}
	prompt := firstPromptText(t, m)
	if prompt != "自定义XEND" {
		t.Errorf("期望 prompt='自定义XEND'，实际=%q", prompt)
	}
}

// TestHistoryAwareRewriter_OnlyRecentHistory 验证 20 条 history 只保留最近 10 条.
func TestHistoryAwareRewriter_OnlyRecentHistory(t *testing.T) {
	m := &mockChatModel{
		generateFn: func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
			return &llm.ChatResponse{Message: llm.AssistantMessage("ok")}, nil
		},
	}
	r := mustHistoryAware(t, m)

	// 20 条消息，编号 msg0..msg19.
	history := make([]llm.Message, 0, 20)
	for i := 0; i < 20; i++ {
		role := llm.RoleUser
		if i%2 == 1 {
			role = llm.RoleAssistant
		}
		history = append(history, llm.Message{Role: role, Content: "msg" + strconv.Itoa(i)})
	}

	_, err := r.Rewrite(context.Background(), "Q", history)
	if err != nil {
		t.Fatalf("Rewrite 失败: %v", err)
	}

	prompt := firstPromptText(t, m)
	// 最近 10 条为 msg10..msg19，应全部出现.
	for i := 10; i < 20; i++ {
		tag := "msg" + strconv.Itoa(i)
		if !strings.Contains(prompt, tag) {
			t.Errorf("prompt 应包含 %s（最近 10 条），实际=%q", tag, prompt)
		}
	}
	// 前 10 条 msg0..msg9 不应出现.
	for i := 0; i < 10; i++ {
		tag := "msg" + strconv.Itoa(i)
		// 注意 msg10..msg19 包含 "msg1"、"msg2"... 作为前缀；使用完整匹配（冒号后紧跟内容 + 换行或字符串结尾）避免误判.
		needle := ": " + tag
		if strings.Contains(prompt, needle+"\n") || strings.HasSuffix(prompt, needle) {
			t.Errorf("prompt 不应包含较早的 %s，实际=%q", tag, prompt)
		}
	}
}

// TestHistoryAwareRewriter_BoundaryHistoryLength 验证历史长度边界：
//   - 恰好 10 条 → 最早的 msg0 仍保留
//   - 11 条 → msg0 被截掉，msg1 是最早保留的
func TestHistoryAwareRewriter_BoundaryHistoryLength(t *testing.T) {
	tests := []struct {
		name       string
		count      int    // history 条数（user/assistant 交替）
		wantOldest string // prompt 应含的最早 msg 文本（使用 "角色: 内容" 完整匹配）
		wantAbsent string // prompt 不应含的 "角色: 内容"（空串表示不检查）
	}{
		{"exactly 10 kept", 10, "msg0", ""},
		{"exactly 11 truncates earliest", 11, "msg1", "msg0"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			history := make([]llm.Message, 0, tc.count)
			for i := 0; i < tc.count; i++ {
				role := llm.RoleUser
				if i%2 == 1 {
					role = llm.RoleAssistant
				}
				history = append(history, llm.Message{Role: role, Content: "msg" + strconv.Itoa(i)})
			}
			m := &mockChatModel{
				generateFn: func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
					return &llm.ChatResponse{Message: llm.AssistantMessage("rewritten")}, nil
				},
			}
			r := mustHistoryAware(t, m)
			_, err := r.Rewrite(context.Background(), "new query", history)
			if err != nil {
				t.Fatalf("Rewrite 失败: %v", err)
			}

			prompt := firstPromptText(t, m)
			// 完整匹配 "角色: 内容"，避免 msg1 误命中 msg10.
			// msg0=user, msg1=assistant（i%2==1 才是 assistant）.
			oldestRole := "用户"
			if indexOfMsg(tc.wantOldest)%2 == 1 {
				oldestRole = "助手"
			}
			wantLine := oldestRole + ": " + tc.wantOldest
			if !lineMatch(prompt, wantLine) {
				t.Errorf("prompt 应包含最早行 %q，实际=\n%s", wantLine, prompt)
			}
			if tc.wantAbsent != "" {
				absentRole := "用户"
				if indexOfMsg(tc.wantAbsent)%2 == 1 {
					absentRole = "助手"
				}
				absentLine := absentRole + ": " + tc.wantAbsent
				if lineMatch(prompt, absentLine) {
					t.Errorf("prompt 不应包含被截掉的行 %q，实际=\n%s", absentLine, prompt)
				}
			}
		})
	}
}

// TestHistoryAwareRewriter_SkipsSystemPreservesCount 验证 system 被跳过后，
// 仍能从剩余非 system 消息中凑满最近 10 条.
func TestHistoryAwareRewriter_SkipsSystemPreservesCount(t *testing.T) {
	history := make([]llm.Message, 0, 17)
	// 2 条 system（前置）+ 15 条 user/assistant 交替.
	history = append(history,
		llm.Message{Role: llm.RoleSystem, Content: "sys1"},
		llm.Message{Role: llm.RoleSystem, Content: "sys2"},
	)
	for i := 0; i < 15; i++ {
		role := llm.RoleUser
		if i%2 == 1 {
			role = llm.RoleAssistant
		}
		history = append(history, llm.Message{Role: role, Content: "m" + strconv.Itoa(i)})
	}
	m := &mockChatModel{
		generateFn: func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
			return &llm.ChatResponse{Message: llm.AssistantMessage("rewritten")}, nil
		},
	}
	r := mustHistoryAware(t, m)
	_, err := r.Rewrite(context.Background(), "new", history)
	if err != nil {
		t.Fatalf("Rewrite 失败: %v", err)
	}

	prompt := firstPromptText(t, m)
	// 15 条非 system 取最近 10 条 → m5..m14；m0..m4 应被截掉，sys1/sys2 始终不出现.
	for _, want := range []string{"m5", "m14"} {
		role := "用户"
		if indexOfMsg(want)%2 == 1 {
			role = "助手"
		}
		line := role + ": " + want
		if !lineMatch(prompt, line) {
			t.Errorf("prompt 应包含 %q，实际=\n%s", line, prompt)
		}
	}
	for _, avoid := range []string{"sys1", "sys2"} {
		if strings.Contains(prompt, avoid) {
			t.Errorf("prompt 不应包含 system 内容 %q，实际=\n%s", avoid, prompt)
		}
	}
	for _, avoid := range []string{"m0", "m1", "m2", "m3", "m4"} {
		role := "用户"
		if indexOfMsg(avoid)%2 == 1 {
			role = "助手"
		}
		line := role + ": " + avoid
		if lineMatch(prompt, line) {
			t.Errorf("prompt 不应包含被截掉的 %q，实际=\n%s", line, prompt)
		}
	}
}

// TestFormatHistory_SkipsSystem 验证 formatHistory 跳过 system 消息.
func TestFormatHistory_SkipsSystem(t *testing.T) {
	history := []llm.Message{
		llm.SystemMessage("你是客服助手"),
		llm.UserMessage("你好"),
		llm.AssistantMessage("您好"),
	}
	got := formatHistory(history)
	if strings.Contains(got, "你是客服助手") {
		t.Errorf("formatHistory 应跳过 system 消息，实际=%q", got)
	}
	if !strings.Contains(got, "用户: 你好") {
		t.Errorf("应保留 user 消息，实际=%q", got)
	}
	if !strings.Contains(got, "助手: 您好") {
		t.Errorf("应保留 assistant 消息，实际=%q", got)
	}
}

// ──────────────────────────────────────────
// HyDERewriter
// ──────────────────────────────────────────

// TestHyDERewriter_GeneratesHypothesis 验证 HyDE 返回 LLM 生成的假设性答案.
func TestHyDERewriter_GeneratesHypothesis(t *testing.T) {
	m := &mockChatModel{
		generateFn: func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
			return &llm.ChatResponse{
				Message: llm.AssistantMessage("VPS 支持 7 天内无理由退款，联系工单提交申请。"),
			}, nil
		},
	}
	r := mustHyDE(t, m)
	got, err := r.Rewrite(context.Background(), "VPS 可以退款吗", nil)
	if err != nil {
		t.Fatalf("Rewrite 失败: %v", err)
	}
	if got != "VPS 支持 7 天内无理由退款，联系工单提交申请。" {
		t.Errorf("期望返回假设答案，实际=%q", got)
	}
	prompt := firstPromptText(t, m)
	if !strings.Contains(prompt, "VPS 可以退款吗") {
		t.Errorf("prompt 应包含原 query，实际=%q", prompt)
	}
}

// TestHyDERewriter_EmptyQuery 验证 query 为空时不调 LLM.
func TestHyDERewriter_EmptyQuery(t *testing.T) {
	m := &mockChatModel{}
	r := mustHyDE(t, m)
	got, err := r.Rewrite(context.Background(), "", nil)
	if err != nil {
		t.Fatalf("期望无错误，实际得到: %v", err)
	}
	if got != "" {
		t.Errorf("期望返回空字符串，实际=%q", got)
	}
	if m.calls != 0 {
		t.Errorf("期望不调用 LLM，实际调用 %d 次", m.calls)
	}
}

// TestHyDERewriter_LLMError 验证 LLM 失败时返回原 query + 非 nil error.
func TestHyDERewriter_LLMError(t *testing.T) {
	sentinel := errors.New("hyde failure")
	m := &mockChatModel{
		generateFn: func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
			return nil, sentinel
		},
	}
	r := mustHyDE(t, m)
	got, err := r.Rewrite(context.Background(), "什么是 RAG", nil)
	if err == nil {
		t.Fatal("期望返回非 nil error")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("期望 error 可被 errors.Is 匹配到底层错误，实际=%v", err)
	}
	if got != "什么是 RAG" {
		t.Errorf("期望返回原 query，实际=%q", got)
	}
}

// TestHyDERewriter_EmptyResponseReturnsOriginal 验证 LLM 空返回时回退原 query.
func TestHyDERewriter_EmptyResponseReturnsOriginal(t *testing.T) {
	m := &mockChatModel{
		generateFn: func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
			return &llm.ChatResponse{Message: llm.AssistantMessage("   ")}, nil
		},
	}
	r := mustHyDE(t, m)
	got, err := r.Rewrite(context.Background(), "原 query", nil)
	if err != nil {
		t.Fatalf("期望无错误，实际得到: %v", err)
	}
	if got != "原 query" {
		t.Errorf("期望返回原 query，实际=%q", got)
	}
}

// TestHyDERewriter_CustomSystemPrompt 验证自定义 prompt 中 {{.Query}} 被替换.
func TestHyDERewriter_CustomSystemPrompt(t *testing.T) {
	m := &mockChatModel{
		generateFn: func(ctx context.Context, messages []llm.Message, opts ...llm.CallOption) (*llm.ChatResponse, error) {
			return &llm.ChatResponse{Message: llm.AssistantMessage("ok")}, nil
		},
	}
	r := mustHyDE(t, m, WithSystemPrompt("[HyDE]{{.Query}}!!"))
	_, err := r.Rewrite(context.Background(), "X", nil)
	if err != nil {
		t.Fatalf("Rewrite 失败: %v", err)
	}
	got := firstPromptText(t, m)
	if got != "[HyDE]X!!" {
		t.Errorf("期望 prompt='[HyDE]X!!'，实际=%q", got)
	}
}

// ──────────────────────────────────────────
// 测试辅助
// ──────────────────────────────────────────

// indexOfMsg 从 "msgN" 或 "mN" 形式的字符串中解析尾部整数，失败返回 0.
// 仅用于边界测试里根据编号奇偶判断角色.
func indexOfMsg(s string) int {
	i := len(s)
	for i > 0 {
		c := s[i-1]
		if c < '0' || c > '9' {
			break
		}
		i--
	}
	if i == len(s) {
		return 0
	}
	n, err := strconv.Atoi(s[i:])
	if err != nil {
		return 0
	}
	return n
}

// lineMatch 判断 prompt 中是否以整行形式出现 line（可能是最后一行或末尾无换行）.
func lineMatch(prompt, line string) bool {
	if strings.Contains(prompt, line+"\n") {
		return true
	}
	if strings.HasSuffix(prompt, line) {
		return true
	}
	return false
}
