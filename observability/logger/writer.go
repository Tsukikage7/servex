package logger

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// RotateWriter 日志轮转写入器接口.
type RotateWriter interface {
	io.Writer
	Sync() error
	Close() error
}

// rotateWriter 按时间轮转的写入器.
type rotateWriter struct {
	baseDir      string
	prefix       string
	maxAge       time.Duration
	compress     bool
	rotationMode string
	location     *time.Location // 时区，默认 UTC

	mu         sync.Mutex
	currentDay string
	file       *os.File
}

// RotateWriterOption 轮转写入器选项.
type RotateWriterOption func(*rotateWriter)

// WithMaxAge 设置最大保留天数.
func WithMaxAge(days int) RotateWriterOption {
	return func(w *rotateWriter) {
		w.maxAge = time.Duration(days) * 24 * time.Hour
	}
}

// WithCompress 设置是否压缩.
func WithCompress(compress bool) RotateWriterOption {
	return func(w *rotateWriter) {
		w.compress = compress
	}
}

// WithRotationMode 设置轮转模式.
func WithRotationMode(mode string) RotateWriterOption {
	return func(w *rotateWriter) {
		w.rotationMode = mode
	}
}

// WithLocation 设置日志轮转使用的时区.
// 默认为 UTC，如需北京时间传入 time.FixedZone("CST", 8*3600) 或 time.LoadLocation("Asia/Shanghai").
func WithLocation(loc *time.Location) RotateWriterOption {
	return func(w *rotateWriter) {
		if loc != nil {
			w.location = loc
		}
	}
}

// NewRotateWriter 创建轮转写入器.
// 默认时区为 UTC，如需指定时区使用 WithLocation 选项.
func NewRotateWriter(baseDir, prefix string, opts ...RotateWriterOption) RotateWriter {
	w := &rotateWriter{
		baseDir:      baseDir,
		prefix:       prefix,
		rotationMode: RotationDaily,
		location:     time.UTC,
	}

	for _, opt := range opts {
		opt(w)
	}

	// 选项应用后再初始化时间戳，确保格式与轮转模式一致
	w.currentDay = w.dirTimestamp()

	return w
}

func (w *rotateWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.shouldRotate() {
		w.rotate()
	}

	if w.file == nil {
		if err := w.openFile(); err != nil {
			return 0, err
		}
	}

	return w.file.Write(p)
}

func (w *rotateWriter) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file != nil {
		return w.file.Sync()
	}
	return nil
}

func (w *rotateWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file != nil {
		err := w.file.Close()
		w.file = nil
		return err
	}
	return nil
}

func (w *rotateWriter) shouldRotate() bool {
	current := w.dirTimestamp()
	return w.currentDay != current
}

func (w *rotateWriter) rotate() {
	if w.file != nil {
		w.file.Close()
		w.file = nil
	}

	w.currentDay = w.dirTimestamp()
	w.cleanupOldLogs()
}

func (w *rotateWriter) openFile() error {
	dir := w.currentDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ErrCreateDir
	}

	filename := w.buildFilename()
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return ErrOpenFile
	}

	w.file = file
	return nil
}

// dirTimestamp 根据轮转模式返回当前时间戳字符串，用作目录名和轮转判断依据.
// 按天：20060102；按小时：2006010215
func (w *rotateWriter) dirTimestamp() string {
	now := time.Now().In(w.location)
	if strings.ToLower(w.rotationMode) == RotationHourly {
		return now.Format("2006010215")
	}
	return now.Format("20060102")
}

// currentDir 返回当前轮转周期对应的目录：baseDir/prefix/20060102 或 baseDir/prefix/2006010215
func (w *rotateWriter) currentDir() string {
	return filepath.Join(w.baseDir, w.prefix, w.currentDay)
}

func (w *rotateWriter) buildFilename() string {
	return filepath.Join(w.currentDir(), w.prefix+".log")
}

func (w *rotateWriter) cleanupOldLogs() {
	if w.maxAge <= 0 {
		return
	}

	// 扫描 baseDir/prefix/ 下的日期子目录（格式 20060102）
	prefixDir := filepath.Join(w.baseDir, w.prefix)
	cutoff := time.Now().Add(-w.maxAge)

	entries, err := os.ReadDir(prefixDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// 支持按天（8位：20060102）和按小时（10位：2006010215）两种目录格式
		name := entry.Name()
		var t time.Time
		var err error
		switch len(name) {
		case 8:
			t, err = time.ParseInLocation("20060102", name, time.Local)
		case 10:
			t, err = time.ParseInLocation("2006010215", name, time.Local)
		default:
			continue
		}
		if err != nil {
			continue
		}
		if t.Before(cutoff) {
			dateDir := filepath.Join(prefixDir, name)
			if w.compress {
				w.compressDir(dateDir)
			} else {
				os.RemoveAll(dateDir)
			}
		}
	}
}

// compressDir 压缩目录内所有未压缩的日志文件后删除目录.
func (w *rotateWriter) compressDir(dir string) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, f := range files {
		if f.IsDir() || isCompressedFile(f.Name()) {
			continue
		}
		w.compressFile(filepath.Join(dir, f.Name()))
	}
	// 若目录已空（压缩完毕），删除目录本身
	remaining, _ := os.ReadDir(dir)
	if len(remaining) == 0 {
		os.Remove(dir)
	}
}


func (w *rotateWriter) compressFile(filename string) {
	input, err := os.Open(filename)
	if err != nil {
		return
	}
	defer input.Close()

	output, err := os.Create(filename + ".gz")
	if err != nil {
		return
	}

	gzWriter := gzip.NewWriter(output)

	if _, err := io.Copy(gzWriter, input); err != nil {
		gzWriter.Close()
		output.Close()
		os.Remove(filename + ".gz")
		return
	}

	// 必须检查 gzWriter.Close() 错误，确保压缩数据完整写入
	if err := gzWriter.Close(); err != nil {
		output.Close()
		os.Remove(filename + ".gz")
		return
	}
	output.Close()

	os.Remove(filename)
}

func isCompressedFile(filename string) bool {
	return strings.HasSuffix(filename, ".gz")
}

// syncWriter 同步写入包装器.
type syncWriter struct {
	writer io.Writer
}

func newSyncWriter(w io.Writer) *syncWriter {
	return &syncWriter{writer: w}
}

func (s *syncWriter) Write(p []byte) (n int, err error) {
	return s.writer.Write(p)
}

func (s *syncWriter) Sync() error {
	if syncer, ok := s.writer.(interface{ Sync() error }); ok {
		return syncer.Sync()
	}
	return nil
}

func (s *syncWriter) Close() error {
	if closer, ok := s.writer.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
