package logshipper

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	es "github.com/elastic/go-elasticsearch/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tsukikage7/servex/messaging/pubsub"
	"github.com/Tsukikage7/servex/observability/logger"
	"github.com/Tsukikage7/servex/storage/elasticsearch"
)

// ────────────────────────────────────────────────────────────────────────────
// Mock: Sink
// ────────────────────────────────────────────────────────────────────────────

type mockSink struct {
	mu      sync.Mutex
	batches [][]Entry
	closed  bool
}

func (m *mockSink) Write(_ context.Context, entries []Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]Entry, len(entries))
	copy(cp, entries)
	m.batches = append(m.batches, cp)
	return nil
}

func (m *mockSink) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockSink) totalEntries() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, b := range m.batches {
		n += len(b)
	}
	return n
}

func (m *mockSink) batchCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.batches)
}

// makeEntry 构造测试用日志条目.
func makeEntry(msg string) Entry {
	return Entry{
		Timestamp: time.Now(),
		Level:     "info",
		Message:   msg,
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Shipper 测试
// ────────────────────────────────────────────────────────────────────────────

// TestShipper_BatchFlush 验证批量 flush：投递 150 条、batchSize=100，应触发 ≥2 次 Write.
func TestShipper_BatchFlush(t *testing.T) {
	sink := &mockSink{}
	s := New(sink,
		WithBatchSize(100),
		WithFlushInterval(10*time.Second), // 禁用定时触发，只靠批量
		WithBufferSize(1000),
	)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	s.Start(ctx)

	for i := range 150 {
		s.Ship(makeEntry("msg"))
		_ = i
	}

	// 等待批量投递完成
	time.Sleep(200 * time.Millisecond)
	cancel()
	// 关闭以确保剩余条目被 flush
	require.NoError(t, s.Close())

	total := sink.totalEntries()
	assert.Equal(t, 150, total, "应投递 150 条日志")
	assert.GreaterOrEqual(t, sink.batchCount(), 2, "应触发至少 2 次批量写入")
}

// TestShipper_FlushInterval 验证定时 flush：投递 10 条后等待 interval，应自动 flush.
func TestShipper_FlushInterval(t *testing.T) {
	sink := &mockSink{}
	interval := 100 * time.Millisecond
	s := New(sink,
		WithBatchSize(1000), // 批量大，不会自动触发
		WithFlushInterval(interval),
		WithBufferSize(1000),
	)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	s.Start(ctx)

	for range 10 {
		s.Ship(makeEntry("msg"))
	}

	// 等待定时器触发
	time.Sleep(interval * 3)
	cancel()
	require.NoError(t, s.Close())

	assert.Equal(t, 10, sink.totalEntries(), "应通过定时 flush 投递 10 条日志")
}

// TestShipper_DropOnFull 验证 dropOnFull=true 时缓冲区满不阻塞.
func TestShipper_DropOnFull(t *testing.T) {
	sink := &mockSink{}
	s := New(sink,
		WithBatchSize(100),
		WithFlushInterval(10*time.Second),
		WithBufferSize(5), // 极小缓冲
		WithDropOnFull(true),
	)
	// 不启动后台协程，channel 很快满

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 10 {
			s.Ship(makeEntry("msg"))
		}
	}()

	select {
	case <-done:
		// 正常：不阻塞
	case <-time.After(2 * time.Second):
		t.Fatal("dropOnFull=true 时 Ship 不应阻塞")
	}
}

// TestShipper_Close 验证 Close 能 flush 剩余条目并关闭 sink.
func TestShipper_Close(t *testing.T) {
	sink := &mockSink{}
	s := New(sink,
		WithBatchSize(1000),
		WithFlushInterval(10*time.Second),
		WithBufferSize(1000),
	)
	ctx, cancel := context.WithCancel(t.Context())
	s.Start(ctx)
	defer cancel()

	for range 50 {
		s.Ship(makeEntry("msg"))
	}

	require.NoError(t, s.Close())
	assert.Equal(t, 50, sink.totalEntries(), "Close 后应 flush 所有 50 条日志")
	assert.True(t, sink.closed, "Close 后 sink 应被关闭")
}

// ────────────────────────────────────────────────────────────────────────────
// Mock: elasticsearch.Client
// ────────────────────────────────────────────────────────────────────────────

// mockESDocument 模拟 elasticsearch.Document.
type mockESDocument struct {
	mu      sync.Mutex
	written []elasticsearch.BulkAction
	indexed []struct {
		id   string
		body any
	}
}

func (m *mockESDocument) Index(_ context.Context, id string, body any) (*elasticsearch.IndexResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.indexed = append(m.indexed, struct {
		id   string
		body any
	}{id, body})
	return &elasticsearch.IndexResult{ID: id, Result: "created"}, nil
}

func (m *mockESDocument) Get(_ context.Context, _ string) (*elasticsearch.GetResult, error) {
	return nil, nil
}

func (m *mockESDocument) Update(_ context.Context, _ string, _ any) (*elasticsearch.UpdateResult, error) {
	return nil, nil
}

func (m *mockESDocument) Delete(_ context.Context, _ string) (*elasticsearch.DeleteResult, error) {
	return nil, nil
}

func (m *mockESDocument) Bulk(_ context.Context, actions []elasticsearch.BulkAction) (*elasticsearch.BulkResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.written = append(m.written, actions...)
	return &elasticsearch.BulkResult{Errors: false}, nil
}

func (m *mockESDocument) Exists(_ context.Context, _ string) (bool, error) {
	return false, nil
}

// mockESIndex 模拟 elasticsearch.Index.
type mockESIndex struct {
	name string
	doc  *mockESDocument
}

func (m *mockESIndex) Create(_ context.Context, _ map[string]any) error      { return nil }
func (m *mockESIndex) Delete(_ context.Context) error                        { return nil }
func (m *mockESIndex) Exists(_ context.Context) (bool, error)                { return true, nil }
func (m *mockESIndex) PutMapping(_ context.Context, _ map[string]any) error  { return nil }
func (m *mockESIndex) GetMapping(_ context.Context) (map[string]any, error)  { return nil, nil }
func (m *mockESIndex) PutSettings(_ context.Context, _ map[string]any) error { return nil }
func (m *mockESIndex) PutAlias(_ context.Context, _ string) error            { return nil }
func (m *mockESIndex) DeleteAlias(_ context.Context, _ string) error         { return nil }
func (m *mockESIndex) Document() elasticsearch.Document                      { return m.doc }
func (m *mockESIndex) Search() elasticsearch.Search                          { return nil }

// mockESClient 模拟 elasticsearch.Client.
type mockESClient struct {
	mu      sync.Mutex
	indices map[string]*mockESIndex
}

func newMockESClient() *mockESClient {
	return &mockESClient{indices: make(map[string]*mockESIndex)}
}

func (m *mockESClient) Index(name string) elasticsearch.Index {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.indices[name]; !ok {
		m.indices[name] = &mockESIndex{name: name, doc: &mockESDocument{}}
	}
	return m.indices[name]
}

func (m *mockESClient) Ping(_ context.Context) error { return nil }
func (m *mockESClient) Close() error                 { return nil }
func (m *mockESClient) Client() *es.Client           { return nil }

// ────────────────────────────────────────────────────────────────────────────
// ES Sink 测试
// ────────────────────────────────────────────────────────────────────────────

// TestElasticsearchSink 验证日志条目写入正确的 ES 索引.
func TestElasticsearchSink(t *testing.T) {
	mock := newMockESClient()

	sink := NewElasticsearchSink(mock,
		WithIndexPrefix("logs-"),
		WithDateSuffix("2006.01.02"),
	)

	fixedTime, _ := time.Parse("2006-01-02", "2026-04-05")
	entries := []Entry{
		{Timestamp: fixedTime, Level: "info", Message: "hello"},
		{Timestamp: fixedTime, Level: "error", Message: "world"},
	}

	ctx := t.Context()
	require.NoError(t, sink.Write(ctx, entries))

	expectedIndex := "logs-2026.04.05"
	mock.mu.Lock()
	idx, ok := mock.indices[expectedIndex]
	mock.mu.Unlock()

	require.True(t, ok, "应写入索引 %s", expectedIndex)
	idx.doc.mu.Lock()
	written := len(idx.doc.written)
	idx.doc.mu.Unlock()
	assert.Equal(t, 2, written, "应写入 2 条文档")
}

// ────────────────────────────────────────────────────────────────────────────
// Mock: pubsub.Publisher
// ────────────────────────────────────────────────────────────────────────────

type mockPublisher struct {
	mu       sync.Mutex
	messages []*pubsub.Message
	topics   []string
}

func (m *mockPublisher) Publish(_ context.Context, topic string, msgs ...*pubsub.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, msg := range msgs {
		m.messages = append(m.messages, msg)
		m.topics = append(m.topics, topic)
	}
	return nil
}

func (m *mockPublisher) Close() error { return nil }

// ────────────────────────────────────────────────────────────────────────────
// Kafka Sink 测试
// ────────────────────────────────────────────────────────────────────────────

// TestKafkaSink 验证日志条目序列化并发布到正确的 topic.
func TestKafkaSink(t *testing.T) {
	pub := &mockPublisher{}
	sink := NewKafkaSink(pub, WithTopic("app-logs"))

	entries := []Entry{
		{Timestamp: time.Now(), Level: "info", Message: "kafka test"},
	}

	require.NoError(t, sink.Write(t.Context(), entries))

	pub.mu.Lock()
	defer pub.mu.Unlock()

	require.Len(t, pub.messages, 1)
	assert.Equal(t, "app-logs", pub.topics[0], "应发布到正确的 topic")

	// 验证消息体是合法 JSON
	var decoded Entry
	require.NoError(t, json.Unmarshal(pub.messages[0].Body, &decoded))
	assert.Equal(t, "kafka test", decoded.Message)
}

// ────────────────────────────────────────────────────────────────────────────
// Mock: logger.Logger
// ────────────────────────────────────────────────────────────────────────────

type mockLogger struct {
	mu       sync.Mutex
	messages []string
}

func (m *mockLogger) record(msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msg)
}

func (m *mockLogger) Debug(args ...any)                           { m.record("debug:" + argsToString(args)) }
func (m *mockLogger) Debugf(f string, args ...any)                { m.record("debug:" + f) }
func (m *mockLogger) Info(args ...any)                            { m.record("info:" + argsToString(args)) }
func (m *mockLogger) Infof(f string, args ...any)                 { m.record("info:" + f) }
func (m *mockLogger) Warn(args ...any)                            { m.record("warn:" + argsToString(args)) }
func (m *mockLogger) Warnf(f string, args ...any)                 { m.record("warn:" + f) }
func (m *mockLogger) Error(args ...any)                           { m.record("error:" + argsToString(args)) }
func (m *mockLogger) Errorf(f string, args ...any)                { m.record("error:" + f) }
func (m *mockLogger) Fatal(args ...any)                           { m.record("fatal:" + argsToString(args)) }
func (m *mockLogger) Fatalf(f string, args ...any)                { m.record("fatal:" + f) }
func (m *mockLogger) Panic(args ...any)                           { m.record("panic:" + argsToString(args)) }
func (m *mockLogger) Panicf(f string, args ...any)                { m.record("panic:" + f) }
func (m *mockLogger) With(_ ...logger.Field) logger.Logger        { return m }
func (m *mockLogger) WithContext(_ context.Context) logger.Logger { return m }
func (m *mockLogger) Sync() error                                 { return nil }
func (m *mockLogger) Close() error                                { return nil }

// ────────────────────────────────────────────────────────────────────────────
// NewLoggerHook 测试
// ────────────────────────────────────────────────────────────────────────────

// TestNewLoggerHook 验证 inner logger 和 shipper 均被调用.
func TestNewLoggerHook(t *testing.T) {
	sink := &mockSink{}
	s := New(sink, WithBatchSize(100), WithFlushInterval(10*time.Second), WithBufferSize(100))
	ctx, cancel := context.WithCancel(t.Context())
	s.Start(ctx)
	defer cancel()

	inner := &mockLogger{}
	hook := NewLoggerHook(inner, s, "info")

	hook.Info("hello")
	hook.Debug("should be skipped") // 低于 minLevel，不投递
	hook.Warn("warn msg")

	// 等待异步投递
	time.Sleep(50 * time.Millisecond)

	// 验证 inner logger 收到所有调用
	inner.mu.Lock()
	msgs := inner.messages
	inner.mu.Unlock()
	require.Len(t, msgs, 3, "inner logger 应收到 3 次调用（info/debug/warn）")

	// 关闭 shipper 以 flush
	require.NoError(t, s.Close())

	// 验证 shipper 只收到 info 和 warn（debug 被过滤）
	assert.Equal(t, 2, sink.totalEntries(), "shipper 应收到 2 条日志（debug 被过滤）")
}
