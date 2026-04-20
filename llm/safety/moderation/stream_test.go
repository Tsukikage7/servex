package moderation_test

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/safety/moderation"
)

// --- 测试辅助 ---

// scriptedReader 按预设片段返回的 StreamReader.
type scriptedReader struct {
	chunks  []string
	delay   time.Duration
	closed  atomic.Bool
	pos     int
	sendErr error
}

func (r *scriptedReader) Recv() (llm.StreamChunk, error) {
	if r.closed.Load() {
		return llm.StreamChunk{}, io.EOF
	}
	if r.sendErr != nil && r.pos >= len(r.chunks) {
		return llm.StreamChunk{}, r.sendErr
	}
	if r.pos >= len(r.chunks) {
		return llm.StreamChunk{}, io.EOF
	}
	if r.delay > 0 {
		time.Sleep(r.delay)
	}
	c := llm.StreamChunk{Delta: r.chunks[r.pos]}
	r.pos++
	return c, nil
}

func (r *scriptedReader) Response() *llm.ChatResponse {
	return &llm.ChatResponse{Message: llm.AssistantMessage("done")}
}

func (r *scriptedReader) Close() error {
	r.closed.Store(true)
	return nil
}

// stubModerator 按调用次数/文本内容决定 Flagged.
type stubModerator struct {
	mu       sync.Mutex
	calls    int
	texts    []string
	trigger  func(string) bool        // 决定是否标记
	reason   string                   // 可选
	delay    time.Duration            // 模拟审核耗时
	onResult func(string, bool)       // 回调,便于测试观察
	err      error                    // 若非 nil 则 Moderate 返回此错误
	scores   map[moderation.Category]float64
}

func (s *stubModerator) Moderate(_ context.Context, text string) (*moderation.Result, error) {
	s.mu.Lock()
	s.calls++
	s.texts = append(s.texts, text)
	s.mu.Unlock()
	if s.delay > 0 {
		time.Sleep(s.delay)
	}
	if s.err != nil {
		return nil, s.err
	}
	flagged := false
	if s.trigger != nil {
		flagged = s.trigger(text)
	}
	if s.onResult != nil {
		s.onResult(text, flagged)
	}
	r := &moderation.Result{
		Flagged:    flagged,
		Categories: map[moderation.Category]bool{moderation.CategoryViolence: flagged},
		Scores:     s.scores,
		Reason:     s.reason,
	}
	return r, nil
}

func (s *stubModerator) ModerateMessages(ctx context.Context, messages []llm.Message) (*moderation.Result, error) {
	// 本测试不使用该入口.
	_ = ctx
	_ = messages
	return &moderation.Result{}, nil
}

func (s *stubModerator) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

// drain 连续 Recv 直到 EOF/错误,返回累积 Delta 与终止错误.
func drain(t *testing.T, r llm.StreamReader) (string, error) {
	t.Helper()
	var out string
	for {
		c, err := r.Recv()
		if err != nil {
			return out, err
		}
		out += c.Delta
	}
}

// --- 测试用例 ---

func TestStreamModerator_CleanContentNotFlagged(t *testing.T) {
	mod := &stubModerator{trigger: func(string) bool { return false }}
	reader := &scriptedReader{chunks: []string{"hello", " world"}}

	sm := moderation.NewStreamModerator(mod,
		moderation.WithChunkChars(3),
		moderation.WithChunkInterval(time.Hour),
	)
	wrapped := sm.Wrap(reader)
	defer wrapped.Close()

	out, err := drain(t, wrapped)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("期望 io.EOF,得到 %v", err)
	}
	if out != "hello world" {
		t.Errorf("期望 'hello world',得到 %q", out)
	}
	// 等待后台 goroutine 完成再断言 call count.
	_ = wrapped.Close()
	// 至少触发过一次审核(字符阈值 3 < 'hello'=5).
	if mod.callCount() == 0 {
		t.Error("期望至少触发一次审核,但未触发")
	}
}

func TestStreamModerator_TriggerAtChunkCharsThreshold(t *testing.T) {
	mod := &stubModerator{trigger: func(string) bool { return false }}
	reader := &scriptedReader{chunks: []string{"abc", "def", "ghi"}}

	sm := moderation.NewStreamModerator(mod,
		moderation.WithChunkChars(5),
		moderation.WithChunkInterval(time.Hour), // 只靠字符阈值触发
	)
	wrapped := sm.Wrap(reader)
	defer wrapped.Close()

	if _, err := drain(t, wrapped); !errors.Is(err, io.EOF) {
		t.Fatalf("drain error: %v", err)
	}
	// 等后台审核完成.
	waitScanDone(t, wrapped)

	if mod.callCount() < 1 {
		t.Errorf("期望至少 1 次审核,得到 %d", mod.callCount())
	}
}

func TestStreamModerator_FlaggedEndsStream(t *testing.T) {
	var flaggedCalls atomic.Int32
	mod := &stubModerator{
		trigger: func(s string) bool { return len(s) > 0 }, // 首次就标记
	}
	reader := &scriptedReader{chunks: []string{"violence here", "more bad", "extra"}}

	sm := moderation.NewStreamModerator(mod,
		moderation.WithChunkChars(1),
		moderation.WithChunkInterval(time.Hour),
		moderation.WithOnFlagged(func(_ *moderation.Result) {
			flaggedCalls.Add(1)
		}),
	)
	wrapped := sm.Wrap(reader)
	defer wrapped.Close()

	// 第一轮 Recv.
	c1, err := wrapped.Recv()
	if err != nil {
		t.Fatalf("第一次 Recv 失败: %v", err)
	}
	if c1.Delta == "" {
		t.Error("期望第一块非空 Delta")
	}
	// 等审核完成(后台 goroutine).
	waitForFlagged(t, wrapped)

	// 第二次 Recv 应返回 io.EOF.
	if _, err := wrapped.Recv(); !errors.Is(err, io.EOF) {
		t.Errorf("命中违规后期望 io.EOF,得到 %v", err)
	}

	if got := flaggedCalls.Load(); got != 1 {
		t.Errorf("期望 onFlagged 被调用 1 次,得到 %d", got)
	}
}

func TestStreamModerator_CloseForwarded(t *testing.T) {
	mod := &stubModerator{trigger: func(string) bool { return false }}
	reader := &scriptedReader{chunks: []string{"ok"}}

	sm := moderation.NewStreamModerator(mod, moderation.WithChunkChars(1_000_000))
	wrapped := sm.Wrap(reader)

	if _, err := wrapped.Recv(); err != nil {
		t.Fatalf("Recv: %v", err)
	}
	if err := wrapped.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !reader.closed.Load() {
		t.Error("Close 未转发到底层 reader")
	}
	// Close 幂等.
	if err := wrapped.Close(); err != nil {
		t.Errorf("二次 Close 失败: %v", err)
	}
}

func TestStreamModerator_NilModeratorPassthrough(t *testing.T) {
	reader := &scriptedReader{chunks: []string{"a", "b"}}
	sm := moderation.NewStreamModerator(nil)
	wrapped := sm.Wrap(reader)

	out, err := drain(t, wrapped)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("drain: %v", err)
	}
	if out != "ab" {
		t.Errorf("期望 'ab',得到 %q", out)
	}
}

// Wrap(nil) 是 programming bug,应 fail-fast 以 panic.
func TestStreamModerator_WrapNilReaderPanics(t *testing.T) {
	sm := moderation.NewStreamModerator(nil)
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("期望 Wrap(nil) panic,但没有 panic")
		}
	}()
	_ = sm.Wrap(nil)
}

// --- 辅助等待函数 ---

// waitForFlagged 轮询等待 moderatedStream 的 flagged 状态,最多等 1s.
func waitForFlagged(t *testing.T, r llm.StreamReader) {
	t.Helper()
	type flaggable interface {
		LastResult() *moderation.Result
	}
	f, ok := r.(flaggable)
	if !ok {
		t.Fatal("wrapped reader 不支持 LastResult")
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if res := f.LastResult(); res != nil && res.Flagged {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("等待 flagged 超时")
}

// waitScanDone 轮询等待后台审核 goroutine 完成.
func waitScanDone(t *testing.T, r llm.StreamReader) {
	t.Helper()
	type flaggable interface {
		LastResult() *moderation.Result
	}
	f, ok := r.(flaggable)
	if !ok {
		return
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if f.LastResult() != nil {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}
