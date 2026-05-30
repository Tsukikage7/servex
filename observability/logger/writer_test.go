package logger

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// RotateWriterTestSuite 轮转写入器测试套件.
type RotateWriterTestSuite struct {
	suite.Suite
	tmpDir string
}

func TestRotateWriterSuite(t *testing.T) {
	suite.Run(t, new(RotateWriterTestSuite))
}

func (s *RotateWriterTestSuite) SetupTest() {
	s.tmpDir = s.T().TempDir()
}

func (s *RotateWriterTestSuite) TestNewRotateWriter() {
	writer := NewRotateWriter(s.tmpDir, "test")
	s.NotNil(writer)
	defer writer.Close()
}

func (s *RotateWriterTestSuite) TestNewRotateWriter_WithOptions() {
	writer := NewRotateWriter(
		s.tmpDir,
		"test",
		WithMaxAge(30),
		WithCompress(true),
		WithRotationMode(RotationHourly),
	)
	s.NotNil(writer)
	defer writer.Close()

	rw := writer.(*rotateWriter)
	s.Equal(30*24*time.Hour, rw.maxAge)
	s.True(rw.compress)
	s.Equal(RotationHourly, rw.rotationMode)
}

func (s *RotateWriterTestSuite) TestWrite() {
	writer := NewRotateWriter(s.tmpDir, "test")
	defer writer.Close()

	data := []byte("test log message\n")
	n, err := writer.Write(data)

	s.NoError(err)
	s.Equal(len(data), n)

	// 验证文件创建新结构：test/20060102/test.log
	dateDir := time.Now().UTC().Format("20060102")
	logFile := filepath.Join(s.tmpDir, "test", dateDir, "test.log")
	content, err := os.ReadFile(logFile)
	s.NoError(err)
	s.Equal(string(data), string(content))
}

func (s *RotateWriterTestSuite) TestMultipleWrites() {
	writer := NewRotateWriter(s.tmpDir, "test")
	defer writer.Close()

	for i := 0; i < 100; i++ {
		data := []byte("test log message\n")
		_, err := writer.Write(data)
		s.NoError(err)
	}

	logDir := filepath.Join(s.tmpDir, "test")
	files, err := os.ReadDir(logDir)
	s.NoError(err)
	s.NotEmpty(files)
}

func (s *RotateWriterTestSuite) TestSync() {
	writer := NewRotateWriter(s.tmpDir, "test")
	defer writer.Close()

	_, err := writer.Write([]byte("test\n"))
	s.NoError(err)

	err = writer.Sync()
	s.NoError(err)
}

func (s *RotateWriterTestSuite) TestSyncWithoutFile() {
	writer := NewRotateWriter(s.tmpDir, "test")
	defer writer.Close()

	// Sync without any writes
	err := writer.Sync()
	s.NoError(err)
}

func (s *RotateWriterTestSuite) TestClose() {
	writer := NewRotateWriter(s.tmpDir, "test")

	_, err := writer.Write([]byte("test\n"))
	s.NoError(err)

	err = writer.Close()
	s.NoError(err)

	// 再次 Close 应该没问题
	err = writer.Close()
	s.NoError(err)
}

func (s *RotateWriterTestSuite) TestFileNaming_Daily() {
	writer := NewRotateWriter(s.tmpDir, "app", WithRotationMode(RotationDaily))
	defer writer.Close()

	_, err := writer.Write([]byte("test\n"))
	s.NoError(err)

	// 新结构：baseDir/app/20060102/app.log
	dateDir := time.Now().UTC().Format("20060102")
	expectedFile := filepath.Join(s.tmpDir, "app", dateDir, "app.log")
	_, err = os.Stat(expectedFile)
	s.NoError(err, "expected file %v not found", expectedFile)
}

func (s *RotateWriterTestSuite) TestFileNaming_Hourly() {
	// writer 默认 UTC，测试期望路径也用 UTC 格式化
	writer := NewRotateWriter(s.tmpDir, "app", WithRotationMode(RotationHourly))
	defer writer.Close()

	_, err := writer.Write([]byte("test\n"))
	s.NoError(err)

	// 小时轮转：目录格式 2006010215UTC，文件名 app.log
	hourDir := time.Now().UTC().Format("2006010215")
	expectedFile := filepath.Join(s.tmpDir, "app", hourDir, "app.log")
	_, err = os.Stat(expectedFile)
	s.NoError(err, "expected file %v not found", expectedFile)
}

func (s *RotateWriterTestSuite) TestConcurrentWrites() {
	writer := NewRotateWriter(s.tmpDir, "concurrent")
	defer writer.Close()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Go(func() {
			for j := 0; j < 100; j++ {
				_, _ = writer.Write([]byte("goroutine write\n"))
			}
		})
	}
	wg.Wait()

	logDir := filepath.Join(s.tmpDir, "concurrent")
	files, err := os.ReadDir(logDir)
	s.NoError(err)
	s.NotEmpty(files)
}

func (s *RotateWriterTestSuite) TestRotate() {
	writer := NewRotateWriter(s.tmpDir, "rotate-test", WithMaxAge(1))
	defer writer.Close()

	// 先正常写入一次，建好今天的目录和文件
	_, err := writer.Write([]byte("day1\n"))
	s.NoError(err)

	today := time.Now().UTC().Format("20060102")
	todayDir := filepath.Join(s.tmpDir, "rotate-test", today)
	_, err = os.Stat(todayDir)
	s.NoError(err, "today dir should exist after first write")

	// 修改 currentDay 为昨天，触发下一次写入时的 rotate
	rw := writer.(*rotateWriter)
	rw.mu.Lock()
	rw.currentDay = time.Now().UTC().Add(-24 * time.Hour).Format("20060102")
	rw.mu.Unlock()

	_, err = writer.Write([]byte("day2\n"))
	s.NoError(err)

	// rotate 后 currentDay 重置为今天，文件仍写在今天目录目录已存在
	_, err = os.Stat(todayDir)
	s.NoError(err, "today dir should still exist after rotate")
}

func (s *RotateWriterTestSuite) TestCleanupOldLogs() {
	writer := NewRotateWriter(s.tmpDir, "cleanup-test", WithMaxAge(1))
	defer writer.Close()

	rw := writer.(*rotateWriter)

	// 创建旧日期目录和日志文件新结构：prefix/20200101/prefix.log
	oldDateDir := filepath.Join(s.tmpDir, "cleanup-test", "20200101")
	err := os.MkdirAll(oldDateDir, 0o755)
	s.Require().NoError(err)
	oldLogFile := filepath.Join(oldDateDir, "cleanup-test.log")
	err = os.WriteFile(oldLogFile, []byte("old log"), 0o644)
	s.Require().NoError(err)

	rw.cleanupOldLogs()

	// 旧日期目录应被整体删除
	_, err = os.Stat(oldDateDir)
	s.True(os.IsNotExist(err), "old date dir should be removed")
}

func (s *RotateWriterTestSuite) TestCleanupOldLogs_WithCompress() {
	writer := NewRotateWriter(s.tmpDir, "compress-test", WithMaxAge(1), WithCompress(true))
	defer writer.Close()

	rw := writer.(*rotateWriter)

	// 创建旧日期目录
	oldDateDir := filepath.Join(s.tmpDir, "compress-test", "20200101")
	err := os.MkdirAll(oldDateDir, 0o755)
	s.Require().NoError(err)
	oldLogFile := filepath.Join(oldDateDir, "compress-test.log")
	err = os.WriteFile(oldLogFile, []byte("old log content"), 0o644)
	s.Require().NoError(err)

	rw.cleanupOldLogs()

	// 原文件应被压缩为 .gz
	_, err = os.Stat(oldLogFile + ".gz")
	s.NoError(err, "compressed file should exist")
	_, err = os.Stat(oldLogFile)
	s.True(os.IsNotExist(err), "original file should be deleted after compression")
}

func (s *RotateWriterTestSuite) TestCleanupOldLogs_SkipNonDateDirs() {
	writer := NewRotateWriter(s.tmpDir, "skipdir-test", WithMaxAge(1))
	defer writer.Close()

	rw := writer.(*rotateWriter)

	// 创建非日期格式的子目录应被跳过
	prefixDir := filepath.Join(s.tmpDir, "skipdir-test")
	err := os.MkdirAll(prefixDir, 0o755)
	s.Require().NoError(err)
	nonDateDir := filepath.Join(prefixDir, "notadate")
	err = os.MkdirAll(nonDateDir, 0o755)
	s.Require().NoError(err)

	s.NotPanics(func() { rw.cleanupOldLogs() })

	_, err = os.Stat(nonDateDir)
	s.NoError(err, "non-date dir should still exist")
}

func (s *RotateWriterTestSuite) TestCleanupOldLogs_NoMaxAge() {
	writer := NewRotateWriter(s.tmpDir, "nomaxage-test")
	defer writer.Close()

	rw := writer.(*rotateWriter)

	// 不设置 maxAge，cleanupOldLogs 应该直接返回
	s.NotPanics(func() {
		rw.cleanupOldLogs()
	})
}

func (s *RotateWriterTestSuite) TestCleanupOldLogs_NonExistentDir() {
	rw := &rotateWriter{
		baseDir: "/nonexistent/path",
		prefix:  "test",
		maxAge:  24 * time.Hour,
	}

	// 不应该 panic
	s.NotPanics(func() {
		rw.cleanupOldLogs()
	})
}

func (s *RotateWriterTestSuite) TestCompressFile() {
	writer := NewRotateWriter(s.tmpDir, "compress-func-test")
	defer writer.Close()

	rw := writer.(*rotateWriter)

	// 创建测试文件
	testFile := filepath.Join(s.tmpDir, "test-compress.log")
	testContent := "test content for compression"
	err := os.WriteFile(testFile, []byte(testContent), 0o644)
	s.Require().NoError(err)

	// 压缩文件
	rw.compressFile(testFile)

	// 验证压缩文件存在
	_, err = os.Stat(testFile + ".gz")
	s.NoError(err, "compressed file should exist")

	// 验证原文件被删除
	_, err = os.Stat(testFile)
	s.True(os.IsNotExist(err), "original file should be deleted")
}

func (s *RotateWriterTestSuite) TestCompressFile_NonExistent() {
	rw := &rotateWriter{}

	// 压缩不存在的文件不应该 panic
	s.NotPanics(func() {
		rw.compressFile("/nonexistent/file.log")
	})
}

func (s *RotateWriterTestSuite) TestShouldRotate_Hourly() {
	writer := NewRotateWriter(s.tmpDir, "hourly-rotate", WithRotationMode(RotationHourly))
	defer writer.Close()

	rw := writer.(*rotateWriter)

	// 写入数据创建文件
	_, err := writer.Write([]byte("test\n"))
	s.NoError(err)

	// 不应该立即需要轮转
	rw.mu.Lock()
	shouldRotate := rw.shouldRotate()
	rw.mu.Unlock()
	s.False(shouldRotate)
}

func (s *RotateWriterTestSuite) TestShouldRotate_DayChange() {
	writer := NewRotateWriter(s.tmpDir, "day-rotate")
	defer writer.Close()

	rw := writer.(*rotateWriter)

	// 修改 currentDay 为昨天
	rw.mu.Lock()
	rw.currentDay = time.Now().UTC().Add(-24 * time.Hour).Format("20060102")
	shouldRotate := rw.shouldRotate()
	rw.mu.Unlock()

	s.True(shouldRotate)
}

func (s *RotateWriterTestSuite) TestBuildFilename() {
	rw := &rotateWriter{
		baseDir:      s.tmpDir,
		prefix:       "test",
		currentDay:   "20240115", // 新格式：无连字符
		rotationMode: RotationDaily,
	}

	// 按天：baseDir/test/20240115/test.log
	filename := rw.buildFilename()
	s.Contains(filename, filepath.Join("test", "20240115", "test.log"))

	// 按小时：baseDir/test/2024011514/test.log目录含小时，文件名统一 prefix.log
	rw.currentDay = "2024011514"
	rw.rotationMode = RotationHourly
	filename = rw.buildFilename()
	s.Contains(filename, filepath.Join("test", "2024011514", "test.log"))
}

// SyncWriterTestSuite 同步写入器测试套件.
type SyncWriterTestSuite struct {
	suite.Suite
	tmpDir string
}

func TestSyncWriterSuite(t *testing.T) {
	suite.Run(t, new(SyncWriterTestSuite))
}

func (s *SyncWriterTestSuite) SetupTest() {
	s.tmpDir = s.T().TempDir()
}

func (s *SyncWriterTestSuite) TestSyncWriter() {
	file, err := os.CreateTemp(s.tmpDir, "test*.log")
	s.Require().NoError(err)
	defer os.Remove(file.Name())
	defer file.Close()

	sw := newSyncWriter(file)

	data := []byte("test message\n")
	n, err := sw.Write(data)
	s.NoError(err)
	s.Equal(len(data), n)

	err = sw.Sync()
	s.NoError(err)

	err = sw.Close()
	s.NoError(err)
}

func (s *SyncWriterTestSuite) TestSyncWriter_NonSyncable() {
	sw := newSyncWriter(&nonSyncableWriter{})

	_, err := sw.Write([]byte("test"))
	s.NoError(err)

	// Sync 应该返回 nilwriter 不支持 Sync
	err = sw.Sync()
	s.NoError(err)

	// Close 应该返回 nilwriter 不支持 Close
	err = sw.Close()
	s.NoError(err)
}

// nonSyncableWriter 用于测试的不支持 Sync 的 writer.
type nonSyncableWriter struct {
	data []byte
}

func (w *nonSyncableWriter) Write(p []byte) (n int, err error) {
	w.data = append(w.data, p...)
	return len(p), nil
}

// HelperFunctionTestSuite 辅助函数测试套件.
type HelperFunctionTestSuite struct {
	suite.Suite
}

func TestHelperFunctionSuite(t *testing.T) {
	suite.Run(t, new(HelperFunctionTestSuite))
}

func (s *HelperFunctionTestSuite) TestIsCompressedFile() {
	testCases := []struct {
		filename string
		want     bool
	}{
		{"app.log", false},
		{"app.log.gz", true},
		{"app.gz", true},
		{"app.tar.gz", true},
		{"app", false},
		{".gz", true},
	}

	for _, tc := range testCases {
		s.Equal(tc.want, isCompressedFile(tc.filename), "filename: %s", tc.filename)
	}
}
