# config/source/etcd

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/config/source/etcd"
```

## 简介

`config/source/etcd` 提供基于 etcd KV 的配置源实现，实现 `config.Source` 和 `config.Watcher` 接口。从指定 etcd key 加载配置，并通过 etcd Watch 机制监听 PUT 事件实现配置热更新。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `Source` | etcd 配置源，实现 `config.Source` |
| `New(client, key, opts...)` | 创建配置源 |
| `WithFormat(format)` | 指定配置格式，默认 `"json"` |
| `Load()` | 从 etcd 加载当前配置值 |
| `Watch()` | 创建 etcd Watch 监听器，阻塞到有 PUT 事件 |

## 示例

```go
package main

import (
    "fmt"
    "time"

    clientv3 "go.etcd.io/etcd/client/v3"

    "github.com/Tsukikage7/servex/v2/config"
    "github.com/Tsukikage7/servex/v2/config/source/etcd"
)

type ServiceConfig struct {
    Timeout int    `json:"timeout"`
    Addr    string `json:"addr"`
}

func main() {
    // 创建 etcd 客户端
    client, err := clientv3.New(clientv3.Config{
        Endpoints:   []string{"localhost:2379"},
        DialTimeout: 5 * time.Second,
    })
    if err != nil {
        panic(err)
    }
    defer client.Close()

    // 创建 etcd 配置源，监听 /config/service 键
    src := etcd.New(client, "/config/service",
        etcd.WithFormat("json"),
    )

    // 加载配置
    kvs, err := src.Load()
    if err != nil {
        panic(err)
    }
    fmt.Println("加载了", len(kvs), "个配置项")

    // 与 config.Manager 配合（支持热更新）
    manager, err := config.NewManager(
        config.WithSource(src),
    )
    if err != nil {
        panic(err)
    }

    var cfg ServiceConfig
    if err := manager.Scan(&cfg); err != nil {
        panic(err)
    }
    fmt.Println("Timeout:", cfg.Timeout)
    fmt.Println("Addr:", cfg.Addr)

    // 监听变更（阻塞，直到有 PUT 事件）
    watcher, _ := src.Watch()
    defer watcher.Stop()

    go func() {
        for {
            newKVs, err := watcher.Next()
            if err != nil {
                return
            }
            fmt.Println("配置更新:", string(newKVs[0].Value))
        }
    }()
}
```
