package moderation

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/Tsukikage7/servex/v2/llm"
)

// 默认触发阈值.
const (
	defaultChunkChars    = 200
	defaultChunkInterval = 500 * time.Millisecond
)

// StreamModerator 流式审核器.
// 包装 llm.StreamReader,在流式生成过程中按字符累积和时间间隔触发审核.
// 当累积字符达到 chunkChars 或距上次审核超过 chunkInterval 时触发一次审核;
// 命中 threshold 时调用 onFlagged 回调,并让后续 Recv 返回 io.EOF 终止流.
type StreamModerator struct {
	moderator     Moderator
	chunkChars    int
	chunkInterval time.Duration
	onFlagged     func(*Result)
}

// StreamOption StreamModerator 构造选项.
type StreamOption func(*StreamModerator)

// WithChunkChars 设置触发审核的字符阈值(默认 200).
// 参数 <= 0 时忽略.
func WithChunkChars(n int) StreamOption {
	return func(s *StreamModerator) {
		if n > 0 {
			s.chunkChars = n
		}
	}
}

// WithChunkInterval 设置触发审核的时间间隔(默认 500ms).
// 参数 <= 0 时忽略.
func WithChunkInterval(d time.Duration) StreamOption {
	return func(s *StreamModerator) {
		if d > 0 {
			s.chunkInterval = d
		}
	}
}

// WithOnFlagged 设置违规命中回调.
// 回调在触发阈值时同步调用,参数为审核器返回的 Result.
func WithOnFlagged(fn func(*Result)) StreamOption {
	return func(s *StreamModerator) {
		s.onFlagged = fn
	}
}

// NewStreamModerator 创建流式审核器包装.
// mod 为底层审核器;传入 nil 时 Wrap 调用 Recv 直接透传原流(不做审核).
func NewStreamModerator(mod Moderator, opts ...StreamOption) *StreamModerator {
	sm := &StreamModerator{
		moderator:     mod,
		chunkChars:    defaultChunkChars,
		chunkInterval: defaultChunkInterval,
	}
	for _, opt := range opts {
		opt(sm)
	}
	return sm
}

// Wrap 包装 StreamReader,返回审核流.
// 审核逻辑:边生成边审,满阈值即后台异步调用底层 Moderator.Moderate;
// 一旦命中 Flagged,后续 Recv 立刻返回 io.EOF,并对原 reader 调用 Close.
// 若底层 Moderator 为 nil,直接返回原 reader 不做审核(无审核能力时行为等同直接透传).
//
// Wrap panics if reader is nil:传 nil 是调用方的 programming bug,
// 应当 fail-fast 而非让下游 defer wrapped.Close() 在隐式 nil 上崩溃.
func (sm *StreamModerator) Wrap(reader llm.StreamReader) llm.StreamReader {
	if reader == nil {
		panic("moderation: StreamModerator.Wrap called with nil reader")
	}
	if sm == nil || sm.moderator == nil {
		return reader
	}
	return &moderatedStream{
		sm:       sm,
		reader:   reader,
		lastScan: time.Now(),
	}
}

// moderatedStream StreamModerator 包装后的流.
// 仅保证 Recv/Response/Close 本身的并发安全;外层约定 Recv 由单一 goroutine 顺序调用(同 llm.StreamReader 契约).
type moderatedStream struct {
	sm       *StreamModerator
	reader   llm.StreamReader
	buffer   []rune
	lastScan time.Time

	mu        sync.Mutex
	flagged   bool    // 违规命中(已回调);后续 Recv 返回 EOF
	lastRes   *Result // 最近一次命中的结果(供测试/观察)
	scanning  bool    // 是否有后台审核 goroutine 正在执行
	scanCh    chan struct{}
	closeOnce sync.Once
	closeErr  error
}

// Recv 读取下一个片段.
// 命中违规后立即返回 io.EOF,并异步关闭原 reader.
func (m *moderatedStream) Recv() (llm.StreamChunk, error) {
	// 命中违规后直接 EOF.
	if m.isFlagged() {
		return llm.StreamChunk{}, io.EOF
	}

	chunk, err := m.reader.Recv()
	if err != nil {
		// 流已结束/出错:取消后台审核并直接透传.
		return chunk, err
	}

	// 累积字符并按阈值触发审核.
	if chunk.Delta != "" {
		m.buffer = append(m.buffer, []rune(chunk.Delta)...)
	}

	if m.shouldScan() {
		m.triggerScan()
	}

	// 审核可能已经(在本轮或下一轮 Recv 前)把状态置为 flagged;再次检查以尽快短路.
	if m.isFlagged() {
		return llm.StreamChunk{}, io.EOF
	}

	return chunk, nil
}

// Response 透传原 reader 的累积响应.
func (m *moderatedStream) Response() *llm.ChatResponse {
	return m.reader.Response()
}

// Close 关闭流.命中违规时幂等关闭底层 reader 并等待后台审核结束.
func (m *moderatedStream) Close() error {
	m.closeOnce.Do(func() {
		m.mu.Lock()
		ch := m.scanCh
		m.mu.Unlock()
		// 等待后台审核完成,避免泄漏 goroutine.
		if ch != nil {
			<-ch
		}
		m.closeErr = m.reader.Close()
	})
	return m.closeErr
}

// LastResult 返回最近一次命中的审核结果(nil 表示未命中或尚未完成首轮审核).
// 仅用于观察/测试,不在接口契约中.
func (m *moderatedStream) LastResult() *Result {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastRes
}

// isFlagged 并发安全读取 flagged 状态.
func (m *moderatedStream) isFlagged() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.flagged
}

// shouldScan 判断是否应当触发一次新审核(字符数满或时间间隔满,且当前无扫描进行).
func (m *moderatedStream) shouldScan() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.scanning || m.flagged {
		return false
	}
	if len(m.buffer) == 0 {
		return false
	}
	if len(m.buffer) >= m.sm.chunkChars {
		return true
	}
	if time.Since(m.lastScan) >= m.sm.chunkInterval {
		return true
	}
	return false
}

// triggerScan 启动一次后台审核.复制当前 buffer 快照,重置 buffer 和计时.
func (m *moderatedStream) triggerScan() {
	m.mu.Lock()
	if m.scanning || m.flagged {
		m.mu.Unlock()
		return
	}
	snapshot := string(m.buffer)
	m.buffer = m.buffer[:0]
	m.lastScan = time.Now()
	m.scanning = true
	done := make(chan struct{})
	m.scanCh = done
	m.mu.Unlock()

	go func() {
		defer close(done)
		// 使用独立 context,Recv 迭代过程取消不影响已启动的审核.
		res, err := m.sm.moderator.Moderate(context.Background(), snapshot)

		m.mu.Lock()
		m.scanning = false
		if err == nil && res != nil {
			m.lastRes = res
			if res.Flagged {
				m.flagged = true
			}
		}
		cb := m.sm.onFlagged
		shouldCallback := err == nil && res != nil && res.Flagged
		m.mu.Unlock()

		if shouldCallback && cb != nil {
			cb(res)
		}
	}()
}

// 编译期接口断言.
var _ llm.StreamReader = (*moderatedStream)(nil)
