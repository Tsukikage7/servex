// Package logshipper 提供日志投递功能，将结构化日志异步投递到外部存储（ES、Kafka 等）.
package logshipper

import (
	"context"
	"sync"
	"time"
)

// Entry 日志条目（结构化 JSON 格式）.
type Entry struct {
	Timestamp  time.Time      `json:"timestamp"`
	Level      string         `json:"level"`
	Message    string         `json:"message"`
	Logger     string         `json:"logger,omitempty"`
	Caller     string         `json:"caller,omitempty"`
	StackTrace string         `json:"stack_trace,omitempty"`
	Fields     map[string]any `json:"fields,omitempty"`
}

// Sink 日志投递目标接口.
type Sink interface {
	// Write 批量写入日志条目
	Write(ctx context.Context, entries []Entry) error
	// Close 关闭投递目标
	Close() error
}

// config 投递器配置.
type config struct {
	batchSize     int
	flushInterval time.Duration
	bufferSize    int
	dropOnFull    bool
	errorHandler  func(error)
}

// Option 投递器选项.
type Option func(*config)

// WithBatchSize 设置批量大小，默认 100.
func WithBatchSize(n int) Option {
	return func(c *config) {
		if n > 0 {
			c.batchSize = n
		}
	}
}

// WithFlushInterval 设置定时刷新间隔，默认 5s.
func WithFlushInterval(d time.Duration) Option {
	return func(c *config) {
		if d > 0 {
			c.flushInterval = d
		}
	}
}

// WithBufferSize 设置缓冲区大小，默认 10000.
func WithBufferSize(n int) Option {
	return func(c *config) {
		if n > 0 {
			c.bufferSize = n
		}
	}
}

// WithDropOnFull 设置缓冲区满时丢弃（true，默认）还是阻塞（false）.
func WithDropOnFull(drop bool) Option {
	return func(c *config) {
		c.dropOnFull = drop
	}
}

// WithErrorHandler 设置投递失败回调.
func WithErrorHandler(fn func(error)) Option {
	return func(c *config) {
		if fn != nil {
			c.errorHandler = fn
		}
	}
}

// Shipper 日志投递器，负责将日志条目异步批量投递到 Sink.
type Shipper struct {
	sink    Sink
	cfg     config
	ch      chan Entry
	flushCh chan chan error
	// stopCh 用于通知后台协程优雅停止
	stopCh chan struct{}
	wg     sync.WaitGroup
	once   sync.Once
}

// New 创建日志投递器.
func New(sink Sink, opts ...Option) *Shipper {
	cfg := config{
		batchSize:     100,
		flushInterval: 5 * time.Second,
		bufferSize:    10000,
		dropOnFull:    true,
		errorHandler:  func(err error) {},
	}
	for _, o := range opts {
		o(&cfg)
	}

	return &Shipper{
		sink:    sink,
		cfg:     cfg,
		ch:      make(chan Entry, cfg.bufferSize),
		flushCh: make(chan chan error, 1),
		stopCh:  make(chan struct{}),
	}
}

// Ship 投递单条日志，写入缓冲 channel.
// 若 dropOnFull=true 且 channel 满，则丢弃该条目；否则阻塞等待.
// 若投递器已关闭，直接丢弃.
func (s *Shipper) Ship(entry Entry) {
	if s.cfg.dropOnFull {
		select {
		case s.ch <- entry:
		case <-s.stopCh:
			// 投递器已关闭，丢弃
		default:
			// 缓冲区满，丢弃
		}
	} else {
		select {
		case s.ch <- entry:
		case <-s.stopCh:
			// 投递器已关闭，丢弃
		}
	}
}

// Start 启动后台投递协程，在 ctx 取消时优雅停止.
// 应在创建 Shipper 后立即调用.
func (s *Shipper) Start(ctx context.Context) {
	s.wg.Add(1)
	go s.run(ctx)
}

// run 后台投递循环，批量读取 channel 并调用 sink.Write.
func (s *Shipper) run(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.cfg.flushInterval)
	defer ticker.Stop()

	batch := make([]Entry, 0, s.cfg.batchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := s.sink.Write(context.Background(), batch); err != nil {
			s.cfg.errorHandler(err)
		}
		batch = batch[:0]
	}

	// drainAndFlush 排空 channel 中所有剩余条目，然后执行最终 flush.
	drainAndFlush := func() {
		for {
			select {
			case entry, ok := <-s.ch:
				if !ok {
					flush()
					return
				}
				batch = append(batch, entry)
				if len(batch) >= s.cfg.batchSize {
					flush()
				}
			default:
				flush()
				return
			}
		}
	}

	for {
		select {
		case entry := <-s.ch:
			batch = append(batch, entry)
			if len(batch) >= s.cfg.batchSize {
				flush()
			}

		case <-ticker.C:
			flush()

		case replyCh := <-s.flushCh:
			// 主动 flush 请求
			flush()
			replyCh <- nil

		case <-s.stopCh:
			// Close() 触发的优雅停止：排空 channel 后退出
			drainAndFlush()
			return

		case <-ctx.Done():
			// 外部 context 取消：排空 channel 后退出
			drainAndFlush()
			return
		}
	}
}

// Flush 立即刷新缓冲区中的所有条目，阻塞直到完成.
func (s *Shipper) Flush(ctx context.Context) error {
	replyCh := make(chan error, 1)
	select {
	case s.flushCh <- replyCh:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-replyCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close 关闭投递器：先通知后台协程刷新剩余日志，等待完成后关闭 sink.
// 可安全多次调用（幂等）.
func (s *Shipper) Close() error {
	var closeErr error
	s.once.Do(func() {
		// 通知后台协程停止，并等待其退出（含剩余条目 flush）
		close(s.stopCh)
		s.wg.Wait()
		closeErr = s.sink.Close()
	})
	return closeErr
}
