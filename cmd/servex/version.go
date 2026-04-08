package main

import "fmt"

// 版本信息，通过 ldflags 注入.
var (
	version   = "dev"
	gitCommit = "unknown"
	buildDate = "unknown"
)

// runVersion 输出版本信息.
func runVersion() {
	fmt.Printf("servex %s\n  commit: %s\n  built:  %s\n", version, gitCommit, buildDate)
}
