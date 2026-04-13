package file

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/Tsukikage7/servex/v2/config"
)

// FileSourceTestSuite 文件配置源测试套件.
type FileSourceTestSuite struct {
	suite.Suite
	tempDir string
}

func TestFileSourceSuite(t *testing.T) {
	suite.Run(t, new(FileSourceTestSuite))
}

func (s *FileSourceTestSuite) SetupSuite() {
	dir, err := os.MkdirTemp("", "file_source_test")
	s.Require().NoError(err)
	s.tempDir = dir
}

func (s *FileSourceTestSuite) TearDownSuite() {
	os.RemoveAll(s.tempDir)
}

func (s *FileSourceTestSuite) writeFile(name, content string) string {
	path := filepath.Join(s.tempDir, name)
	err := os.WriteFile(path, []byte(content), 0644)
	s.Require().NoError(err)
	return path
}

// === Load 测试 ===

func (s *FileSourceTestSuite) TestLoad_Success() {
	path := s.writeFile("config.yaml", "name: test")
	src := New(path)

	kvs, err := src.Load()
	s.NoError(err)
	s.Len(kvs, 1)
	s.Equal(path, kvs[0].Key)
	s.Equal([]byte("name: test"), kvs[0].Value)
	s.Equal("yaml", kvs[0].Format)
}

func (s *FileSourceTestSuite) TestLoad_JSONFormat() {
	path := s.writeFile("config.json", `{"name":"test"}`)
	src := New(path)

	kvs, err := src.Load()
	s.NoError(err)
	s.Len(kvs, 1)
	s.Equal("json", kvs[0].Format)
}

func (s *FileSourceTestSuite) TestLoad_WithFormat() {
	path := s.writeFile("config.txt", "name: test")
	src := New(path, WithFormat("yaml"))

	kvs, err := src.Load()
	s.NoError(err)
	s.Equal("yaml", kvs[0].Format)
}

func (s *FileSourceTestSuite) TestLoad_FileNotFound() {
	src := New("/nonexistent/file.yaml")

	_, err := src.Load()
	s.Error(err)
}

func (s *FileSourceTestSuite) TestLoad_FormatInference() {
	tests := []struct {
		name     string
		expected string
	}{
		{"test.yaml", "yaml"},
		{"test.yml", "yaml"},
		{"test.json", "json"},
		{"test.toml", "toml"},
		{"test.txt", ""},
	}

	for _, tc := range tests {
		path := s.writeFile(tc.name, "content")
		src := New(path)
		kvs, err := src.Load()
		s.NoError(err)
		s.Equal(tc.expected, kvs[0].Format, "file: %s", tc.name)
	}
}

// === Watch 测试 ===

func (s *FileSourceTestSuite) TestWatch_FileModified() {
	path := s.writeFile("watch_test.yaml", "version: 1")
	src := New(path)

	watcher, err := src.Watch()
	s.Require().NoError(err)
	defer watcher.Stop()

	// 异步修改文件
	go func() {
		time.Sleep(200 * time.Millisecond)
		os.WriteFile(path, []byte("version: 2"), 0644)
	}()

	// 等待变更通知
	done := make(chan struct{})
	var kvs []*config.KeyValue
	var watchErr error
	go func() {
		kvs, watchErr = watcher.Next()
		close(done)
	}()

	select {
	case <-done:
		s.NoError(watchErr)
		s.Len(kvs, 1)
		s.Equal([]byte("version: 2"), kvs[0].Value)
	case <-time.After(3 * time.Second):
		s.Fail("watch timeout")
	}
}

func (s *FileSourceTestSuite) TestWatch_Stop() {
	path := s.writeFile("watch_stop.yaml", "data: test")
	src := New(path)

	watcher, err := src.Watch()
	s.Require().NoError(err)

	// 异步 stop
	go func() {
		time.Sleep(100 * time.Millisecond)
		watcher.Stop()
	}()

	// Next 应因 stop 返回错误
	done := make(chan struct{})
	var watchErr error
	go func() {
		_, watchErr = watcher.Next()
		close(done)
	}()

	select {
	case <-done:
		s.Error(watchErr)
	case <-time.After(3 * time.Second):
		s.Fail("stop timeout")
	}
}

func (s *FileSourceTestSuite) TestWatch_DoubleStop() {
	path := s.writeFile("watch_double_stop.yaml", "data: test")
	src := New(path)

	watcher, err := src.Watch()
	s.Require().NoError(err)

	err = watcher.Stop()
	s.NoError(err)

	// 重复 Stop 不 panic
	err = watcher.Stop()
	// fsnotify watcher 已关闭，可能返回错误但不应 panic
	_ = err
}

func (s *FileSourceTestSuite) TestWatch_NonExistentDirectory() {
	// 文件在不存在的目录中，Watch 应返回错误
	src := New("/nonexistent/dir/config.yaml")

	_, err := src.Watch()
	s.Error(err)
}

func (s *FileSourceTestSuite) TestWatch_FileCreated() {
	// 监听一个尚不存在的文件（但目录存在）
	path := filepath.Join(s.tempDir, "watch_create.yaml")
	src := New(path)

	watcher, err := src.Watch()
	s.Require().NoError(err)
	defer watcher.Stop()

	// 异步创建文件
	go func() {
		time.Sleep(200 * time.Millisecond)
		os.WriteFile(path, []byte("created: true"), 0644)
	}()

	done := make(chan struct{})
	var kvs []*config.KeyValue
	var watchErr error
	go func() {
		kvs, watchErr = watcher.Next()
		close(done)
	}()

	select {
	case <-done:
		s.NoError(watchErr)
		s.Len(kvs, 1)
		s.Contains(string(kvs[0].Value), "created")
	case <-time.After(3 * time.Second):
		s.Fail("watch create timeout")
	}
}

func (s *FileSourceTestSuite) TestWatch_IgnoresOtherFiles() {
	// 修改同目录下的其他文件不应触发通知
	path := s.writeFile("target.yaml", "target: true")
	otherPath := filepath.Join(s.tempDir, "other.yaml")
	src := New(path)

	watcher, err := src.Watch()
	s.Require().NoError(err)
	defer watcher.Stop()

	// 修改其他文件
	os.WriteFile(otherPath, []byte("other: true"), 0644)

	// 然后修改目标文件
	go func() {
		time.Sleep(200 * time.Millisecond)
		os.WriteFile(otherPath, []byte("other: v2"), 0644)
		time.Sleep(100 * time.Millisecond)
		os.WriteFile(path, []byte("target: v2"), 0644)
	}()

	done := make(chan struct{})
	var kvs []*config.KeyValue
	go func() {
		kvs, _ = watcher.Next()
		close(done)
	}()

	select {
	case <-done:
		s.Len(kvs, 1)
		s.Contains(string(kvs[0].Value), "target: v2")
	case <-time.After(3 * time.Second):
		s.Fail("timeout")
	}
}
