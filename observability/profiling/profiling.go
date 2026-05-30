// Package profiling 提供持续性能剖析，支持 CPU/内存/Goroutine/Block/Mutex 的周期采集和上报.
package profiling

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"path/filepath"
	"runtime"
	rpprof "runtime/pprof"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ProfileType 剖析类型.
type ProfileType string

const (
	// ProfileCPU CPU 剖析.
	ProfileCPU ProfileType = "cpu"
	// ProfileHeap 堆内存剖析.
	ProfileHeap ProfileType = "heap"
	// ProfileGoroutine Goroutine 剖析.
	ProfileGoroutine ProfileType = "goroutine"
	// ProfileBlock 阻塞剖析.
	ProfileBlock ProfileType = "block"
	// ProfileMutex 互斥锁剖析.
	ProfileMutex ProfileType = "mutex"
	// ProfileAllocs 内存分配剖析.
	ProfileAllocs ProfileType = "allocs"
	// ProfileThreadCreate 线程创建剖析.
	ProfileThreadCreate ProfileType = "threadcreate"
)

var (
	// ErrNilConfig 配置为空.
	ErrNilConfig = errors.New("profiling: 配置为空")
	// ErrAlreadyRunning 剖析器已在运行.
	ErrAlreadyRunning = errors.New("profiling: 已在运行")
	// ErrNotRunning 剖析器未运行.
	ErrNotRunning = errors.New("profiling: 未运行")
	// ErrInvalidProfileType 无效的剖析类型.
	ErrInvalidProfileType = errors.New("profiling: 无效的剖析类型")
)

// validProfileTypes 合法的剖析类型集合.
var validProfileTypes = map[ProfileType]bool{
	ProfileCPU:          true,
	ProfileHeap:         true,
	ProfileGoroutine:    true,
	ProfileBlock:        true,
	ProfileMutex:        true,
	ProfileAllocs:       true,
	ProfileThreadCreate: true,
}

// Config 持续剖析配置.
type Config struct {
	// Enabled 是否启用.
	Enabled bool
	// Types 采集的剖析类型.
	Types []ProfileType
	// Interval 采集间隔默认 60s.
	Interval time.Duration
	// Duration CPU 剖析每次采集时长默认 10s.
	Duration time.Duration
	// OutputDir 本地保存目录可选.
	OutputDir string
	// Labels 附加标签如 service、env.
	Labels map[string]string
	// BlockProfileRate 阻塞剖析采样率默认 1000000，即每百万次阻塞事件采样一次.
	// 设为 1 可获得最精确的数据，但生产环境开销较大.
	BlockProfileRate int
	// MutexProfileFraction 互斥锁剖析采样分数默认 100.
	MutexProfileFraction int
}

// DefaultConfig 返回默认配置.
func DefaultConfig() *Config {
	return &Config{
		Enabled:              true,
		Types:                []ProfileType{ProfileCPU, ProfileHeap, ProfileGoroutine},
		Interval:             60 * time.Second,
		Duration:             10 * time.Second,
		Labels:               make(map[string]string),
		BlockProfileRate:     1000000, // 每百万次阻塞事件采样一次，适用于生产环境
		MutexProfileFraction: 100,
	}
}

// Exporter 剖析数据导出接口，用于自定义上报后端.
type Exporter interface {
	Export(ctx context.Context, profile *Profile) error
}

// Profile 采集到的剖析数据.
type Profile struct {
	// Type 剖析类型.
	Type ProfileType
	// Data 二进制剖析数据.
	Data []byte
	// Timestamp 采集时间.
	Timestamp time.Time
	// Duration 采集时长仅 CPU 有意义.
	Duration time.Duration
	// Labels 附加标签.
	Labels map[string]string
}

// Status 剖析器运行状态.
type Status struct {
	// Running 是否运行中.
	Running bool
	// LastCollected 最近一次采集时间.
	LastCollected time.Time
	// CollectedCount 累计采集次数.
	CollectedCount int64
	// ErrorCount 累计错误次数.
	ErrorCount int64
	// ActiveProfiles 正在采集的剖析类型.
	ActiveProfiles []ProfileType
}

// Option 配置选项函数.
type Option func(*Profiler)

// WithLogger 设置日志记录器.
func WithLogger(printf func(format string, v ...any)) Option {
	return func(p *Profiler) {
		if printf != nil {
			p.printf = printf
		}
	}
}

// WithExporter 设置剖析数据导出器.
func WithExporter(e Exporter) Option {
	return func(p *Profiler) {
		if e != nil {
			p.exporter = e
		}
	}
}

// WithHTTPPrefix 设置 pprof HTTP 路径前缀默认 "/debug/pprof".
func WithHTTPPrefix(prefix string) Option {
	return func(p *Profiler) {
		if prefix != "" {
			p.httpPrefix = prefix
		}
	}
}

// Profiler 持续性能剖析器.
type Profiler struct {
	config     *Config
	printf     func(format string, v ...any)
	exporter   Exporter
	httpPrefix string

	mu             sync.RWMutex
	running        bool
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	lastCollected  time.Time
	collectedCount atomic.Int64
	errorCount     atomic.Int64
}

// New 创建剖析器.
func New(cfg *Config, opts ...Option) (*Profiler, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}

	// 验证剖析类型
	for _, t := range cfg.Types {
		if !validProfileTypes[t] {
			return nil, fmt.Errorf("%w: %s", ErrInvalidProfileType, t)
		}
	}

	// 填充默认值
	if cfg.Interval <= 0 {
		cfg.Interval = 60 * time.Second
	}
	if cfg.Duration <= 0 {
		cfg.Duration = 10 * time.Second
	}
	if cfg.Labels == nil {
		cfg.Labels = make(map[string]string)
	}

	p := &Profiler{
		config:     cfg,
		printf:     func(string, ...any) {},
		httpPrefix: "/debug/pprof",
	}

	for _, opt := range opts {
		opt(p)
	}

	return p, nil
}

// Start 启动周期采集.
func (p *Profiler) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return ErrAlreadyRunning
	}

	if !p.config.Enabled {
		p.printf("profiling: 未启用，跳过启动")
		return nil
	}

	// 开启 block 和 mutex 剖析，使用可配置的采样率
	for _, t := range p.config.Types {
		switch t {
		case ProfileBlock:
			rate := p.config.BlockProfileRate
			if rate <= 0 {
				rate = 1000000
			}
			runtime.SetBlockProfileRate(rate)
		case ProfileMutex:
			frac := p.config.MutexProfileFraction
			if frac <= 0 {
				frac = 100
			}
			runtime.SetMutexProfileFraction(frac)
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	p.running = true
	p.wg.Go(func() {
		p.loop(ctx)
	})

	p.printf("profiling: 已启动，间隔=%s, 类型=%v", p.config.Interval, p.config.Types)
	return nil
}

// Stop 停止周期采集.
func (p *Profiler) Stop(_ context.Context) error {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return ErrNotRunning
	}

	p.cancel()
	p.running = false
	p.mu.Unlock()

	// 等待采集 goroutine 退出.
	p.wg.Wait()

	// 仅当本 Profiler 启动时设置过 block/mutex 剖析率时才重置
	if p.config.BlockProfileRate > 0 {
		runtime.SetBlockProfileRate(0)
	}
	if p.config.MutexProfileFraction > 0 {
		runtime.SetMutexProfileFraction(0)
	}

	p.printf("profiling: 已停止")
	return nil
}

// Collect 单次采集指定类型的剖析数据.
func (p *Profiler) Collect(ctx context.Context, profileType ProfileType) (*Profile, error) {
	if !validProfileTypes[profileType] {
		return nil, fmt.Errorf("%w: %s", ErrInvalidProfileType, profileType)
	}

	var buf bytes.Buffer
	var duration time.Duration

	switch profileType {
	case ProfileCPU:
		duration = p.config.Duration
		if err := rpprof.StartCPUProfile(&buf); err != nil {
			return nil, fmt.Errorf("profiling: 启动 CPU 剖析失败: %w", err)
		}
		select {
		case <-time.After(duration):
		case <-ctx.Done():
			rpprof.StopCPUProfile()
			return nil, ctx.Err()
		}
		rpprof.StopCPUProfile()
	default:
		prof := rpprof.Lookup(string(profileType))
		if prof == nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidProfileType, profileType)
		}
		if err := prof.WriteTo(&buf, 0); err != nil {
			return nil, fmt.Errorf("profiling: 采集 %s 失败: %w", profileType, err)
		}
	}

	labels := make(map[string]string, len(p.config.Labels))
	for k, v := range p.config.Labels {
		labels[k] = v
	}

	return &Profile{
		Type:      profileType,
		Data:      buf.Bytes(),
		Timestamp: time.Now(),
		Duration:  duration,
		Labels:    labels,
	}, nil
}

// Handler 返回 pprof HTTP 处理器.
func (p *Profiler) Handler() http.Handler {
	mux := http.NewServeMux()
	prefix := strings.TrimRight(p.httpPrefix, "/")

	mux.HandleFunc(prefix+"/", pprof.Index)
	mux.HandleFunc(prefix+"/cmdline", pprof.Cmdline)
	mux.HandleFunc(prefix+"/profile", pprof.Profile)
	mux.HandleFunc(prefix+"/symbol", pprof.Symbol)
	mux.HandleFunc(prefix+"/trace", pprof.Trace)

	return mux
}

// Status 返回当前状态.
func (p *Profiler) Status() *Status {
	p.mu.RLock()
	defer p.mu.RUnlock()

	activeTypes := make([]ProfileType, len(p.config.Types))
	copy(activeTypes, p.config.Types)

	return &Status{
		Running:        p.running,
		LastCollected:  p.lastCollected,
		CollectedCount: p.collectedCount.Load(),
		ErrorCount:     p.errorCount.Load(),
		ActiveProfiles: activeTypes,
	}
}

// loop 周期采集循环.
func (p *Profiler) loop(ctx context.Context) {
	ticker := time.NewTicker(p.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.collectAll(ctx)
		}
	}
}

// collectAll 采集所有配置的剖析类型.
// 注意：CPU profiling 是全局操作runtime/pprof.StartCPUProfile，串行采集避免冲突.
func (p *Profiler) collectAll(ctx context.Context) {
	for _, profileType := range p.config.Types {
		prof, err := p.Collect(ctx, profileType)
		if err != nil {
			p.errorCount.Add(1)
			p.printf("profiling: 采集 %s 失败: %v", profileType, err)
			continue
		}

		p.collectedCount.Add(1)
		p.mu.Lock()
		p.lastCollected = prof.Timestamp
		p.mu.Unlock()

		// 导出
		if p.exporter != nil {
			if err := p.exporter.Export(ctx, prof); err != nil {
				p.errorCount.Add(1)
				p.printf("profiling: 导出 %s 失败: %v", profileType, err)
			}
		}
	}
}

// FileExporter 文件导出器，将剖析数据保存到本地目录.
type FileExporter struct {
	dir string
}

// NewFileExporter 创建文件导出器.
func NewFileExporter(dir string) *FileExporter {
	return &FileExporter{dir: dir}
}

// Export 将剖析数据写入文件.
func (e *FileExporter) Export(_ context.Context, profile *Profile) error {
	if err := os.MkdirAll(e.dir, 0o750); err != nil {
		return fmt.Errorf("profiling: 创建目录失败: %w", err)
	}

	filename := fmt.Sprintf("%s_%s.pprof",
		profile.Type,
		profile.Timestamp.Format("20060102_150405"),
	)
	path := filepath.Join(e.dir, filename)

	if err := os.WriteFile(path, profile.Data, 0o600); err != nil {
		return fmt.Errorf("profiling: 写入文件失败: %w", err)
	}

	return nil
}
