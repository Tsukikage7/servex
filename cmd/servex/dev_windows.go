//go:build windows

package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "开发模式运行（文件变更自动重启）[Windows 暂不支持]",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("servex dev 暂不支持 Windows，请使用 WSL")
	},
}
