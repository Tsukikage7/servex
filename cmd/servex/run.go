package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
)

// detectEntryPoint 自动检测 main 包入口路径.
// 优先级: ./cmd/server > ./cmd/<dirname> > .
func detectEntryPoint() string {
	// 优先检查 cmd/server/main.go
	if _, err := os.Stat("cmd/server/main.go"); err == nil {
		return "./cmd/server"
	}

	// 获取当前目录名，检查 cmd/<dirname>/main.go
	dir, err := os.Getwd()
	if err == nil {
		dirname := filepath.Base(dir)
		candidate := filepath.Join("cmd", dirname, "main.go")
		if _, err := os.Stat(candidate); err == nil {
			return "./cmd/" + dirname
		}
	}

	return "."
}

// runRun 执行 servex run 命令.
func runRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	entry := fs.String("entry", "", "Override entry point (default: auto-detect)")
	race := fs.Bool("race", false, "Enable race detector (-race)")
	fs.Usage = func() {
		fmt.Println("Usage: servex run [options] [-- args...]")
		fmt.Println()
		fmt.Println("Run the current project with go run.")
		fmt.Println("Auto-detects the main package entry point:")
		fmt.Println("  1. ./cmd/server   (if cmd/server/main.go exists)")
		fmt.Println("  2. ./cmd/<dirname> (if cmd/<dirname>/main.go exists)")
		fmt.Println("  3. .              (fallback)")
		fmt.Println()
		fmt.Println("Options:")
		fs.PrintDefaults()
	}

	// 分离 -- 之后的参数
	var goRunArgs, passthrough []string
	for i, a := range args {
		if a == "--" {
			goRunArgs = args[:i]
			passthrough = args[i+1:]
			break
		}
	}
	if passthrough == nil {
		goRunArgs = args
	}

	if err := fs.Parse(goRunArgs); err != nil {
		return err
	}

	entryPoint := *entry
	if entryPoint == "" {
		entryPoint = detectEntryPoint()
	}

	fmt.Printf("servex: running %s...\n", entryPoint)

	// 构建 go run 命令参数
	cmdArgs := []string{"run"}
	if *race {
		cmdArgs = append(cmdArgs, "-race")
	}
	cmdArgs = append(cmdArgs, entryPoint)
	cmdArgs = append(cmdArgs, passthrough...)

	cmd := exec.Command("go", cmdArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// 转发信号到子进程
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start: %w", err)
	}

	go func() {
		sig := <-sigCh
		if cmd.Process != nil {
			_ = cmd.Process.Signal(sig)
		}
	}()

	err := cmd.Wait()
	signal.Stop(sigCh)

	if err != nil {
		// 被信号终止是正常退出场景，不视为错误
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("run: %w", err)
	}
	return nil
}
