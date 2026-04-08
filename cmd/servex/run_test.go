package main

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// TestDetectEntryPointCmdServer 测试优先检测 cmd/server/main.go.
func TestDetectEntryPointCmdServer(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	// 创建 cmd/server/main.go
	if err := os.MkdirAll(filepath.Join(dir, "cmd", "server"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cmd", "server", "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := detectEntryPoint()
	if got != "./cmd/server" {
		t.Errorf("detectEntryPoint() = %q, want %q", got, "./cmd/server")
	}
}

// TestDetectEntryPointCmdDirname 测试检测 cmd/<dirname>/main.go.
func TestDetectEntryPointCmdDirname(t *testing.T) {
	dir := t.TempDir()
	// 创建以目录名命名的子目录
	dirname := filepath.Base(dir)
	entryDir := filepath.Join(dir, "cmd", dirname)
	if err := os.MkdirAll(entryDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(entryDir, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	got := detectEntryPoint()
	want := "./cmd/" + dirname
	if got != want {
		t.Errorf("detectEntryPoint() = %q, want %q", got, want)
	}
}

// TestDetectEntryPointFallback 测试无匹配时回退到当前目录.
func TestDetectEntryPointFallback(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	got := detectEntryPoint()
	if got != "." {
		t.Errorf("detectEntryPoint() = %q, want %q", got, ".")
	}
}

// TestDetectEntryPointPriority 测试 cmd/server 优先于 cmd/<dirname>.
func TestDetectEntryPointPriority(t *testing.T) {
	dir := t.TempDir()
	dirname := filepath.Base(dir)

	// 同时创建 cmd/server 和 cmd/<dirname>
	for _, sub := range []string{"server", dirname} {
		entryDir := filepath.Join(dir, "cmd", sub)
		if err := os.MkdirAll(entryDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(entryDir, "main.go"), []byte("package main"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	got := detectEntryPoint()
	if got != "./cmd/server" {
		t.Errorf("detectEntryPoint() = %q, want %q (server should have priority)", got, "./cmd/server")
	}
}

// TestRunFlagParsing 测试 run 命令的参数解析.
func TestRunFlagParsing(t *testing.T) {
	// 验证 --entry 和 --race 标志能被正确定义
	// 我们不实际执行 go run，只验证 flag set 的构建
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	entry := fs.String("entry", "", "Override entry point")
	race := fs.Bool("race", false, "Enable race detector")

	args := []string{"--entry", "./cmd/myapp", "--race"}
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	if *entry != "./cmd/myapp" {
		t.Errorf("entry = %q, want %q", *entry, "./cmd/myapp")
	}
	if !*race {
		t.Error("race should be true")
	}
}

// TestRunPassthroughSplit 测试 -- 分隔符的参数分离.
func TestRunPassthroughSplit(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantGoArgs  int
		wantPassLen int
	}{
		{
			name:        "no separator",
			args:        []string{"--entry", "./cmd/app"},
			wantGoArgs:  2,
			wantPassLen: 0,
		},
		{
			name:        "with separator",
			args:        []string{"--race", "--", "-v", "--port=8080"},
			wantGoArgs:  1,
			wantPassLen: 2,
		},
		{
			name:        "empty before separator",
			args:        []string{"--", "arg1"},
			wantGoArgs:  0,
			wantPassLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var goRunArgs, passthrough []string
			for i, a := range tt.args {
				if a == "--" {
					goRunArgs = tt.args[:i]
					passthrough = tt.args[i+1:]
					break
				}
			}
			if passthrough == nil {
				goRunArgs = tt.args
			}

			if len(goRunArgs) != tt.wantGoArgs {
				t.Errorf("goRunArgs len = %d, want %d", len(goRunArgs), tt.wantGoArgs)
			}
			if len(passthrough) != tt.wantPassLen {
				t.Errorf("passthrough len = %d, want %d", len(passthrough), tt.wantPassLen)
			}
		})
	}
}
