package logger

import (
	"compress/gzip"
	"fmt"
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

// NewRotateWriter 创建轮转写入器.
func NewRotateWriter(baseDir, prefix string, opts ...RotateWriterOption) RotateWriter {
	w := &rotateWriter{
		baseDir:      baseDir,
		prefix:       prefix,
		rotationMode: RotationDaily,
		currentDay:   time.Now().Format("2006-01-02"),
	}

	for _, opt := range opts {
		opt(w)
	}

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
	now := time.Now()
	today := now.Format("2006-01-02")

	if w.currentDay != today {
		return true
	}

	if strings.ToLower(w.rotationMode) == RotationHourly && w.file != nil {
		stat, err := w.file.Stat()
		if err != nil {
			return false
		}
		return stat.ModTime().Hour() != now.Hour() || stat.ModTime().Day() != now.Day()
	}

	return false
}

func (w *rotateWriter) rotate() {
	if w.file != nil {
		w.file.Close()
		w.file = nil
	}

	w.currentDay = time.Now().Format("2006-01-02")
	w.cleanupOldLogs()
}

func (w *rotateWriter) openFile() error {
	dir := filepath.Join(w.baseDir, w.prefix)
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

func (w *rotateWriter) buildFilename() string {
	dir := filepath.Join(w.baseDir, w.prefix)
	now := time.Now()

	var filename string
	switch strings.ToLower(w.rotationMode) {
	case RotationHourly:
		filename = fmt.Sprintf("%s_%s_%02d.log", w.prefix, w.currentDay, now.Hour())
	default:
		filename = fmt.Sprintf("%s_%s.log", w.prefix, w.currentDay)
	}

	return filepath.Join(dir, filename)
}

func (w *rotateWriter) cleanupOldLogs() {
	if w.maxAge <= 0 {
		return
	}

	dir := filepath.Join(w.baseDir, w.prefix)
	cutoff := time.Now().Add(-w.maxAge)

	files, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filename := file.Name()
		if !w.isLogFile(filename) {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			oldPath := filepath.Join(dir, filename)
			if w.compress && !isCompressedFile(filename) {
				w.compressFile(oldPath)
			} else if !w.compress || info.ModTime().Before(cutoff.Add(-24*time.Hour)) {
				os.Remove(oldPath)
			}
		}
	}
}

func (w *rotateWriter) isLogFile(filename string) bool {
	prefix := w.prefix + "_"
	return strings.HasPrefix(filename, prefix) &&
		(strings.HasSuffix(filename, ".log") || strings.HasSuffix(filename, ".log.gz"))
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
