package main

import (
	"os"
	"path/filepath"
)

// serviceDir 返回新的 monorepo 服务目录。
func serviceDir(name string) string {
	return filepath.Join("services", name)
}

// resolveServiceDirForWrite 返回服务生成目标目录。
func resolveServiceDirForWrite(name string) string {
	return serviceDir(name)
}

// serviceDirExists 判断服务目录是否存在。
func serviceDirExists(name string) (string, bool) {
	dir := serviceDir(name)
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return dir, true
	}
	return "", false
}
