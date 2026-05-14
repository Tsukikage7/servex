# xutil/randx

## 导入路径

```go
import "github.com/Tsukikage7/servex/v2/xutil/randx"
```

## 简介

`xutil/randx` 提供随机数和随机字符串生成工具。`Rand` 封装标准库 `math/rand`，`New(seed)` 用于可重复测试；`NewSecure()` 基于 `crypto/rand` 提供密码学安全的随机数。支持生成随机整数、指定字符集字符串、随机元素采样和切片洗牌。

## 核心类型

| 类型 / 函数 | 说明 |
|---|---|
| `Rand` | 随机数生成器 |
| `New(seed)` | 创建基于种子的随机生成器（可重复） |
| `NewSecure()` | 创建密码学安全随机生成器 |
| `Rand.RandInt(min, max)` | 生成 [min, max) 范围整数 |
| `Rand.RandInt64(min, max)` | 生成 int64 范围整数 |
| `Rand.RandString(n, charset)` | 从指定字符集生成长度为 n 的随机字符串 |
| `Rand.RandAlphanumeric(n)` | 生成字母数字混合字符串 |
| `Rand.RandAlpha(n)` | 生成纯字母字符串 |
| `Rand.RandDigits(n)` | 生成纯数字字符串 |
| `Rand.RandElement(slice)` | 从切片中随机取一个元素 |
| `Rand.Sample(slice, k)` | 从切片中随机取 k 个不重复元素 |
| `Rand.Shuffle(slice)` | 就地随机打乱切片 |

## 示例

```go
package main

import (
    "fmt"

    "github.com/Tsukikage7/servex/v2/xutil/randx"
)

func main() {
    // 普通随机（可重复，适合测试）
    r := randx.New(42)
    fmt.Println("随机整数:", r.RandInt(1, 100))      // [1, 100) 内随机整数
    fmt.Println("随机字母数字:", r.RandAlphanumeric(16)) // 如："aB3xKp9mZq1wYvN2"
    fmt.Println("随机数字串:", r.RandDigits(6))         // 验证码：如 "847291"

    // 密码学安全随机（适合 token、密钥生成）
    secure := randx.NewSecure()
    token := secure.RandAlphanumeric(32)
    fmt.Println("安全 Token:", token)

    // 从切片随机取元素
    colors := []string{"红", "橙", "黄", "绿", "蓝", "紫"}
    fmt.Println("随机颜色:", r.RandElement(colors))

    // 随机采样 3 个不重复元素
    sample := r.Sample(colors, 3)
    fmt.Println("随机样本:", sample)

    // 打乱切片
    nums := []int{1, 2, 3, 4, 5, 6, 7, 8}
    r.Shuffle(nums)
    fmt.Println("打乱后:", nums)

    // 自定义字符集
    hex := r.RandString(8, "0123456789abcdef")
    fmt.Println("随机十六进制:", hex)
}
```
