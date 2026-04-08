package main

import (
	"fmt"
	"os"
	"os/exec"
)

// installPath 是 go install 的目标模块路径，提取为变量以便测试.
var installPath = "github.com/Tsukikage7/servex/cmd/servex@latest"

// runUpgrade 执行 servex upgrade 命令.
func runUpgrade(args []string) error {
	fmt.Println("servex: 正在升级到最新版本...")

	cmd := exec.Command("go", "install", installPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("升级失败: %w", err)
	}

	fmt.Println("servex: 升级成功")

	// 显示新版本
	verCmd := exec.Command("servex", "version")
	verCmd.Stdout = os.Stdout
	verCmd.Stderr = os.Stderr
	// 忽略版本命令的错误，升级本身已成功
	_ = verCmd.Run()

	return nil
}
