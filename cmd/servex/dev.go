package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
)

var (
	devEntry   string
	devWatch   string
	devExclude string
)

// devCmd 开发模式运行命令（文件变更自动重启）.
var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "开发模式运行（文件变更自动重启）",
	RunE:  func(cmd *cobra.Command, args []string) error { return runDev(args) },
}

// devRunner 管理子进程的生命周期.
type devRunner struct {
	entry string
	mu    sync.Mutex
	cmd   *exec.Cmd
}

// start 启动子进程.
func (r *devRunner) start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cmd := exec.Command("go", "run", r.entry)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// 使用进程组以便杀掉子进程树
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动子进程失败: %w", err)
	}
	r.cmd = cmd

	// 异步等待子进程结束，避免僵尸进程
	go func() { _ = cmd.Wait() }()

	return nil
}

// stop 终止子进程.
func (r *devRunner) stop() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cmd == nil || r.cmd.Process == nil {
		return
	}

	// 发送 SIGTERM 到进程组
	_ = syscall.Kill(-r.cmd.Process.Pid, syscall.SIGTERM)

	// 等待子进程退出，超时后强制杀掉
	done := make(chan struct{})
	go func() {
		_ = r.cmd.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		_ = syscall.Kill(-r.cmd.Process.Pid, syscall.SIGKILL)
	}

	r.cmd = nil
}

// restart 重启子进程.
func (r *devRunner) restart() error {
	r.stop()
	return r.start()
}

// runDev 执行 dev 命令.
func runDev(args []string) error {
	entry := devEntry
	if entry == "" {
		entry = detectEntryPoint()
	}

	watchPaths := splitAndTrim(devWatch, ",")
	excludePatterns := splitAndTrim(devExclude, ",")

	fmt.Printf("\033[36m[dev]\033[0m 开发模式启动，入口: %s\n", entry)
	fmt.Printf("\033[36m[dev]\033[0m 监听路径: %s\n", strings.Join(watchPaths, ", "))
	fmt.Printf("\033[36m[dev]\033[0m 排除模式: %s\n", strings.Join(excludePatterns, ", "))

	runner := &devRunner{entry: entry}
	if err := runner.start(); err != nil {
		return err
	}

	// 创建文件监听器
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		runner.stop()
		return fmt.Errorf("创建文件监听器失败: %w", err)
	}
	defer watcher.Close()

	// 递归添加监听路径
	for _, wp := range watchPaths {
		if err := addWatchRecursive(watcher, wp, excludePatterns); err != nil {
			runner.stop()
			return fmt.Errorf("添加监听路径 %q 失败: %w", wp, err)
		}
	}

	// 处理信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	// debounce 定时器
	var debounce *time.Timer
	const debounceDelay = 500 * time.Millisecond

	for {
		select {
		case sig := <-sigCh:
			fmt.Printf("\n\033[36m[dev]\033[0m 收到信号 %v，停止中...\n", sig)
			runner.stop()
			return nil

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			// 只关注 .go 文件变更
			if !strings.HasSuffix(event.Name, ".go") {
				continue
			}

			// 排除匹配的路径
			if shouldExclude(event.Name, excludePatterns) {
				continue
			}

			// 只关注写入/创建/删除/重命名
			if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) &&
				!event.Has(fsnotify.Remove) && !event.Has(fsnotify.Rename) {
				continue
			}

			// 新创建的目录需要加入监听
			if event.Has(fsnotify.Create) {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					_ = addWatchRecursive(watcher, event.Name, excludePatterns)
				}
			}

			// debounce: 重置定时器
			if debounce != nil {
				debounce.Stop()
			}
			debounce = time.AfterFunc(debounceDelay, func() {
				fmt.Printf("\033[33m[dev]\033[0m 文件变更检测到，重启中...\n")
				if err := runner.restart(); err != nil {
					fmt.Printf("\033[31m[dev]\033[0m 重启失败: %v\n", err)
				}
			})

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Printf("\033[31m[dev]\033[0m 监听错误: %v\n", err)
		}
	}
}

// addWatchRecursive 递归添加目录到监听器.
func addWatchRecursive(watcher *fsnotify.Watcher, root string, excludes []string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过无法访问的路径
		}
		if !info.IsDir() {
			return nil
		}
		if shouldExclude(path, excludes) {
			return filepath.SkipDir
		}
		return watcher.Add(path)
	})
}

// shouldExclude 检查路径是否匹配排除模式.
func shouldExclude(path string, patterns []string) bool {
	for _, p := range patterns {
		// 简单的路径包含检查
		if strings.Contains(path, string(filepath.Separator)+p+string(filepath.Separator)) ||
			strings.HasPrefix(path, p+string(filepath.Separator)) ||
			strings.HasSuffix(path, string(filepath.Separator)+p) ||
			path == p {
			return true
		}
	}
	return false
}

// splitAndTrim 分割字符串并去除空白.
func splitAndTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
