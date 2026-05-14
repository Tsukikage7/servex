# xutil/idgen

分布式 ID 生成器，支持 Snowflake、ULID、NanoID 和 UUID 四种算法。

## 特性

- **Snowflake**：41-bit 时间戳 + 5-bit 数据中心 + 5-bit 工作节点 + 12-bit 序列号，纯数字字符串，趋势递增
- **ULID**：26 字符 Crockford Base32，同毫秒内单调递增，字典序可排序
- **NanoID**：可配置字母表和长度，URL 安全，默认 21 字符
- **UUID**：封装 `google/uuid`，标准 v4 格式

所有实现均线程安全，提供统一的 `Generator` 接口和开箱即用的便捷函数。

## 快速开始（便捷函数）

```go
import "github.com/Tsukikage7/servex/v2/xutil/idgen"

id := idgen.Snowflake() // "1234567890123456789"（纯数字）
id  = idgen.ULID()      // "01ARZ3NDEKTSV4RRFFQ69G5FAV"（26 字符）
id  = idgen.NanoID()    // "V1StGXR8_Z5jdHi6B-myT"（21 字符）
id  = idgen.UUID()      // "550e8400-e29b-41d4-a716-446655440000"
```

## Snowflake 生成器

```go
gen, err := idgen.NewSnowflake(&idgen.SnowflakeConfig{
    WorkerID:     1,   // 0-1023（组合了 datacenter + worker）
    DatacenterID: 0,   // 0-31
    Epoch:        time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), // 自定义纪元
})
if err != nil {
    panic(err)
}

id, err := gen.NextID()
```

## ULID 生成器

```go
gen := idgen.NewULID()
id, err := gen.NextID() // 同毫秒内自动单调递增
```

## NanoID 生成器

```go
// 默认：21 字符，字母表 A-Za-z0-9_-
gen := idgen.NewNanoID()

// 自定义
gen = idgen.NewNanoID(
    idgen.WithAlphabet("0123456789abcdef"), // 十六进制字母表
    idgen.WithSize(32),                     // 32 字符长度
)

id, err := gen.NextID()
```

## Generator 接口

```go
type Generator interface {
    NextID() (string, error)
}
```

可将任意生成器注入到业务代码，便于测试替换。

## 算法对比

| 算法 | 长度 | 格式 | 排序性 | 适用场景 |
|---|---|---|---|---|
| Snowflake | ~19 字符 | 纯数字 | 趋势递增 | 数据库主键 |
| ULID | 26 字符 | Base32 | 字典序可排序 | 分布式事件 ID |
| NanoID | 可配置 | 可配置 | 随机 | URL slug、短码 |
| UUID | 36 字符 | 十六进制 | 随机 | 通用唯一标识 |
