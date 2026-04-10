// Package audit 提供结构化审计日志记录，记录谁对什么做了什么.
package audit

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ErrNilStore store 不能为空.
var ErrNilStore = errors.New("audit: store is nil")

// Entry 审计条目.
type Entry struct {
	ID         string            `json:"id" gorm:"primaryKey"`
	Actor      string            `json:"actor" gorm:"index"`
	Action     string            `json:"action" gorm:"index"`
	Resource   string            `json:"resource" gorm:"index"`
	ResourceID string            `json:"resource_id"`
	Changes    map[string]Change `json:"changes,omitzero" gorm:"serializer:json"`
	Metadata   map[string]any    `json:"metadata,omitzero" gorm:"serializer:json"`
	IP         string            `json:"ip"`
	UserAgent  string            `json:"user_agent"`
	CreatedAt  time.Time         `json:"created_at" gorm:"autoCreateTime;index"`
}

// Change 字段变更.
type Change struct {
	From any `json:"from"`
	To   any `json:"to"`
}

// Logger 审计日志记录器.
type Logger interface {
	Log(ctx context.Context, entry *Entry) error
	Query(ctx context.Context, filter *Filter) ([]Entry, error)
}

// Filter 查询过滤.
type Filter struct {
	Actor      string
	Action     string
	Resource   string
	ResourceID string
	From       time.Time
	To         time.Time
	Limit      int
	Offset     int
}

// Store 存储接口.
type Store interface {
	Save(ctx context.Context, entry *Entry) error
	Query(ctx context.Context, filter *Filter) ([]Entry, error)
	AutoMigrate(ctx context.Context) error
}

// Option 配置选项.
type Option func(*options)

type options struct {
	async      bool
	bufferSize int
}

// WithAsync 开启异步写入.
func WithAsync(bufferSize int) Option {
	return func(o *options) {
		o.async = true
		o.bufferSize = bufferSize
	}
}

// logger 审计日志记录器实现.
type logger struct {
	store  Store
	opts   options
	ch     chan *Entry
	done   chan struct{}
	closed bool
	mu     sync.Mutex
}

// NewLogger 创建审计日志记录器.
func NewLogger(store Store, opts ...Option) Logger {
	o := options{}
	for _, opt := range opts {
		opt(&o)
	}

	l := &logger{
		store: store,
		opts:  o,
		done:  make(chan struct{}),
	}

	if o.async {
		if o.bufferSize <= 0 {
			o.bufferSize = 1024
		}
		l.ch = make(chan *Entry, o.bufferSize)
		go l.processAsync()
	}

	return l
}

// processAsync 异步处理审计日志.
func (l *logger) processAsync() {
	defer close(l.done)
	for entry := range l.ch {
		_ = l.store.Save(context.Background(), entry)
	}
}

// Log 记录审计日志.
func (l *logger) Log(ctx context.Context, entry *Entry) error {
	if l.store == nil {
		return ErrNilStore
	}
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}

	if l.opts.async {
		l.mu.Lock()
		if l.closed {
			l.mu.Unlock()
			return errors.New("audit: logger is closed")
		}
		l.mu.Unlock()
		l.ch <- entry
		return nil
	}

	return l.store.Save(ctx, entry)
}

// Query 查询审计日志.
func (l *logger) Query(ctx context.Context, filter *Filter) ([]Entry, error) {
	if l.store == nil {
		return nil, ErrNilStore
	}
	return l.store.Query(ctx, filter)
}

// --- Memory Store ---

type memoryStore struct {
	mu      sync.RWMutex
	entries []Entry
}

// NewMemoryStore 创建基于内存的审计日志存储（用于测试）.
func NewMemoryStore() Store {
	return &memoryStore{}
}

func (s *memoryStore) Save(_ context.Context, entry *Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, *entry)
	return nil
}

func (s *memoryStore) Query(_ context.Context, filter *Filter) ([]Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Entry
	for _, e := range s.entries {
		if filter != nil {
			if filter.Actor != "" && e.Actor != filter.Actor {
				continue
			}
			if filter.Action != "" && e.Action != filter.Action {
				continue
			}
			if filter.Resource != "" && e.Resource != filter.Resource {
				continue
			}
			if filter.ResourceID != "" && e.ResourceID != filter.ResourceID {
				continue
			}
			if !filter.From.IsZero() && e.CreatedAt.Before(filter.From) {
				continue
			}
			if !filter.To.IsZero() && e.CreatedAt.After(filter.To) {
				continue
			}
		}
		result = append(result, e)
	}

	// 应用分页
	if filter != nil {
		if filter.Offset > 0 && filter.Offset < len(result) {
			result = result[filter.Offset:]
		} else if filter.Offset >= len(result) {
			return nil, nil
		}
		if filter.Limit > 0 && filter.Limit < len(result) {
			result = result[:filter.Limit]
		}
	}

	return result, nil
}

func (s *memoryStore) AutoMigrate(_ context.Context) error {
	return nil
}

// --- HTTP 中间件 ---

// MiddlewareOption HTTP 中间件选项.
type MiddlewareOption func(*middlewareOptions)

type middlewareOptions struct {
	actorExtractor    func(r *http.Request) string
	resourceExtractor func(r *http.Request) (string, string)
}

// WithActorExtractor 设置操作者提取函数.
func WithActorExtractor(fn func(r *http.Request) string) MiddlewareOption {
	return func(o *middlewareOptions) {
		o.actorExtractor = fn
	}
}

// WithResourceExtractor 设置资源提取函数.
func WithResourceExtractor(fn func(r *http.Request) (resource, resourceID string)) MiddlewareOption {
	return func(o *middlewareOptions) {
		o.resourceExtractor = fn
	}
}

// HTTPMiddleware 创建审计日志 HTTP 中间件.
func HTTPMiddleware(l Logger, opts ...MiddlewareOption) func(http.Handler) http.Handler {
	o := &middlewareOptions{
		actorExtractor: func(r *http.Request) string {
			return r.Header.Get("X-User-ID")
		},
		resourceExtractor: func(r *http.Request) (string, string) {
			return r.URL.Path, ""
		},
	}
	for _, opt := range opts {
		opt(o)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resource, resourceID := o.resourceExtractor(r)
			entry := &Entry{
				Actor:      o.actorExtractor(r),
				Action:     r.Method,
				Resource:   resource,
				ResourceID: resourceID,
				IP:         r.RemoteAddr,
				UserAgent:  r.UserAgent(),
			}
			_ = l.Log(r.Context(), entry)
			next.ServeHTTP(w, r)
		})
	}
}
