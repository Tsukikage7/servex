package file_test

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Tsukikage7/servex/v2/config/source/file"
)

func ExampleNew() {
	// 创建临时配置文件
	dir, _ := os.MkdirTemp("", "example")
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte("name: example\nport: 8080\n"), 0644)

	// 创建文件配置源
	src := file.New(path)

	// 加载配置
	kvs, _ := src.Load()
	fmt.Println(kvs[0].Format)
	fmt.Println(string(kvs[0].Value))
	// Output:
	// yaml
	// name: example
	// port: 8080
}

func ExampleNew_withFormat() {
	dir, _ := os.MkdirTemp("", "example")
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "config.txt")
	os.WriteFile(path, []byte(`{"port":3000}`), 0644)

	// 显式指定格式
	src := file.New(path, file.WithFormat("json"))

	kvs, _ := src.Load()
	fmt.Println(kvs[0].Format)
	// Output: json
}
