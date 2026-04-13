package main

import (
	"testing"
)

// TestUpgradeInstallPath 验证 go install 目标路径正确.
func TestUpgradeInstallPath(t *testing.T) {
	want := "github.com/Tsukikage7/servex/v2/cmd/servex@latest"
	if installPath != want {
		t.Errorf("installPath = %q, want %q", installPath, want)
	}
}

// TestUpgradeCommandArgs 测试 upgrade 命令构建正确的 go install 参数.
func TestUpgradeCommandArgs(t *testing.T) {
	args := []string{"go", "install", installPath}

	if args[0] != "go" {
		t.Errorf("args[0] = %q, want %q", args[0], "go")
	}
	if args[1] != "install" {
		t.Errorf("args[1] = %q, want %q", args[1], "install")
	}
	if args[2] != "github.com/Tsukikage7/servex/v2/cmd/servex@latest" {
		t.Errorf("args[2] = %q, want module path with @latest", args[2])
	}
}
