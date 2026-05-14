# config/source/env

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/config/source/env"
```

## 简介

`config/source/env` 提供基于环境变量的配置源实现，实现 `config.Source` 接口。支持前缀过滤（自动去除前缀）、`.env` 文件加载（通过 `fsnotify` 监听文件变化）。环境变量序列化为 JSON 格式后交由 `config` 包解析。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `Source` | 环境变量配置源，实现 `config.Source` |
| `New(opts...)` | 创建配置源 |
| `WithPrefix(prefix)` | 仅读取指定前缀的环境变量，并去除前缀 |
| `WithEnvFile(path)` | 指定 `.env` 文件，Watch 时监听文件变化 |

## 示例

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/Tsukikage7/servex/v2/config"
    "github.com/Tsukikage7/servex/v2/config/source/env"
)

type AppConfig struct {
    Port string `json:"APP_PORT"`
    DB   string `json:"APP_DB_DSN"`
}

func main() {
    // 设置环境变量
    os.Setenv("APP_PORT", "8080")
    os.Setenv("APP_DB_DSN", "postgres://localhost/mydb")
    os.Setenv("OTHER_VAR", "ignored")

    // 只读取 APP_ 前缀的变量
    src := env.New(
        env.WithPrefix("APP_"),
    )

    // 加载配置
    kvs, err := src.Load()
    if err != nil {
        panic(err)
    }
    fmt.Println("加载了", len(kvs), "个 KeyValue")

    // 与 config.Manager 配合使用
    manager, err := config.NewManager(
        config.WithSource(src),
    )
    if err != nil {
        panic(err)
    }

    var cfg AppConfig
    if err := manager.Scan(&cfg); err != nil {
        panic(err)
    }
    fmt.Println("Port:", cfg.Port)   // 8080
    fmt.Println("DB:", cfg.DB)       // postgres://localhost/mydb

    // 监听 .env 文件变化（配合文件热加载）
    srcWithFile := env.New(
        env.WithEnvFile(".env"),
        env.WithPrefix("APP_"),
    )
    watcher, _ := srcWithFile.Watch()
    _ = watcher
    _ = context.Background()
}
```
