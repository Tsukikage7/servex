# Cache

统一的缓存接口库。根包提供缓存抽象、内存实现和 provider 注册表；Redis 实现在 `storage/cache/redis` 子包中按需注册，避免只使用内存缓存或接口时拉入 Redis SDK。

## 特性

- **统一接口**：不同缓存后端使用相同的 API
- **按需依赖**：Redis provider 独立在 `storage/cache/redis`
- **分布式锁**：基于 Redis 的安全分布式锁实现
- **原子操作**：SetNX、Increment、Decrement 等
- **批量操作**：MGet、MSet 支持
- **自动过期**：TTL 支持和过期清理
- **强制日志**：必须提供 logger，确保日志不会静默丢失

## 安装

```bash
go get github.com/Tsukikage7/servex/v2/storage/cache
```

## 配置选项

### 完整配置

```go
config := &cache.Config{
    // 基础配置
    Type:     cache.TypeMemory,  // memory；redis 需导入 storage/cache/redis 注册
    Addr:     "localhost:6379",  // Redis 地址
    Password: "",                // Redis 密码
    DB:       0,                 // Redis 数据库编号

    // 连接池配置（Redis）
    PoolSize:     10,                  // 连接池大小
    Timeout:      5 * time.Second,     // 连接超时
    ReadTimeout:  3 * time.Second,     // 读取超时
    WriteTimeout: 3 * time.Second,     // 写入超时
    MaxRetries:   3,                   // 最大重试次数

    // 内存缓存配置
    MaxSize:         10000,            // 最大缓存条目数
    CleanupInterval: time.Minute,      // 过期清理间隔
}
```

### Redis provider

```go
import (
    "github.com/Tsukikage7/servex/v2/storage/cache"
    _ "github.com/Tsukikage7/servex/v2/storage/cache/redis"
)

c, err := cache.NewCache(&cache.Config{
    Type: cache.TypeRedis,
    Addr: "127.0.0.1:6379",
}, log)
```

### 配置说明

| 配置项            | 类型     | 默认值  | 说明             |
| ----------------- | -------- | ------- | ---------------- |
| `Type`            | string   | `redis` | 缓存类型         |
| `Addr`            | string   | -       | Redis 连接地址   |
| `Password`        | string   | -       | Redis 密码       |
| `DB`              | int      | `0`     | Redis 数据库编号 |
| `PoolSize`        | int      | `10`    | 连接池大小       |
| `Timeout`         | Duration | `5s`    | 连接超时         |
| `ReadTimeout`     | Duration | `3s`    | 读取超时         |
| `WriteTimeout`    | Duration | `3s`    | 写入超时         |
| `MaxRetries`      | int      | `3`     | 最大重试次数     |
| `MaxSize`         | int      | `10000` | 内存缓存最大条目 |
| `CleanupInterval` | Duration | `1m`    | 清理间隔         |

## API 参考

### 基础操作

```go
// 设置键值对
err := c.Set(ctx, "key", "value", time.Hour)

// 获取值
value, err := c.Get(ctx, "key")
if err == cache.ErrNotFound {
    // 键不存在
}

// 删除键（支持批量）
err := c.Del(ctx, "key1", "key2", "key3")

// 检查键是否存在
exists, err := c.Exists(ctx, "key")
```

### 原子操作

```go
// 仅当键不存在时设置（用于分布式锁、幂等性）
ok, err := c.SetNX(ctx, "key", "value", time.Hour)
if ok {
    // 设置成功
}

// 递增
val, err := c.Increment(ctx, "counter")

// 增加指定值
val, err := c.IncrementBy(ctx, "counter", 10)

// 递减
val, err := c.Decrement(ctx, "counter")
```

### 过期时间

```go
// 设置过期时间
err := c.Expire(ctx, "key", time.Hour)

// 获取剩余过期时间
ttl, err := c.TTL(ctx, "key")
// ttl > 0: 剩余时间
// ttl == -1: 永不过期
// ttl == -2: 键不存在
```

### 分布式锁

```go
import "github.com/google/uuid"

lockKey := "order:123:lock"
lockValue := uuid.New().String() // 使用 UUID 作为锁值

// 尝试获取锁
ok, err := c.TryLock(ctx, lockKey, lockValue, 30*time.Second)
if !ok {
    // 获取锁失败，已被其他进程持有
    return
}

defer func() {
    // 释放锁（只有持有者才能释放）
    if err := c.Unlock(ctx, lockKey, lockValue); err != nil {
        if err == cache.ErrLockNotHeld {
            // 锁已过期或被其他进程释放
        }
    }
}()

// 执行需要锁保护的操作
doSomething()
```

### 批量操作

```go
// 批量获取
values, err := c.MGet(ctx, "key1", "key2", "key3")
// values[i] 为空字符串表示键不存在

// 批量设置
pairs := map[string]any{
    "key1": "value1",
    "key2": map[string]int{"a": 1},
    "key3": 123,
}
err := c.MSet(ctx, pairs, time.Hour)
```

### 资源管理

```go
// 测试连接
err := c.Ping(ctx)

// 关闭连接
err := c.Close()

// 获取底层客户端（类型断言）
redisClient := c.Client().(*redis.Client)
```

## 日志要求

cache 包**强制要求**提供 `logger.Logger` 实例，不提供会返回 `ErrNilLogger` 错误：

```go
import (
    "github.com/Tsukikage7/servex/v2/cache"
    "github.com/Tsukikage7/servex/v2/observability/logger"
)

// 创建 logger（必需）
log, _ := logger.NewLogger(logger.NewDevConfig())

// logger 作为必需参数传入
c, err := cache.New(config, log)
if err != nil {
    // 可能是 ErrNilLogger、ErrNilConfig 等
    panic(err)
}
```

这种设计确保日志不会被静默丢弃，便于问题排查。

## 最佳实践

### 1. 缓存键命名规范

```go
// 使用冒号分隔的层级结构
"user:123"              // 用户信息
"user:123:profile"      // 用户资料
"order:456:status"      // 订单状态
"lock:order:456"        // 分布式锁
"rate:api:user:123"     // 限流计数
```

### 2. 错误处理

```go
value, err := c.Get(ctx, key)
if err != nil {
    if err == cache.ErrNotFound {
        // 缓存未命中，从数据库加载
        value = loadFromDB(key)
        c.Set(ctx, key, value, time.Hour)
    } else {
        // 缓存错误，记录日志
        log.Error("cache error", logger.Err(err))
        return err
    }
}
```

### 3. 缓存穿透防护

```go
// 使用 SetNX 防止缓存击穿
func GetUserWithLock(ctx context.Context, c cache.Cache, userID string) (*User, error) {
    key := "user:" + userID
    lockKey := "lock:" + key

    // 尝试从缓存获取
    data, err := c.Get(ctx, key)
    if err == nil {
        return parseUser(data), nil
    }

    // 获取锁，防止大量请求同时查询数据库
    lockValue := uuid.New().String()
    ok, _ := c.TryLock(ctx, lockKey, lockValue, 10*time.Second)
    if !ok {
        // 等待其他请求加载
        time.Sleep(100 * time.Millisecond)
        data, _ = c.Get(ctx, key)
        return parseUser(data), nil
    }
    defer c.Unlock(ctx, lockKey, lockValue)

    // 再次检查缓存
    data, err = c.Get(ctx, key)
    if err == nil {
        return parseUser(data), nil
    }

    // 从数据库加载
    user := loadUserFromDB(userID)
    c.Set(ctx, key, user, time.Hour)
    return user, nil
}
```

### 4. 限流器

```go
func RateLimit(ctx context.Context, c cache.Cache, key string, limit int64, window time.Duration) bool {
    count, err := c.Increment(ctx, key)
    if err != nil {
        return false
    }

    if count == 1 {
        c.Expire(ctx, key, window)
    }

    return count <= limit
}

// 使用示例
if !RateLimit(ctx, c, "rate:api:user:123", 100, time.Minute) {
    return errors.New("rate limit exceeded")
}
```

### 5. 优雅关闭

```go
func main() {
    c, _ := cache.New(config)

    // 信号处理
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

    go func() {
        <-sigCh
        c.Close()
        os.Exit(0)
    }()

    // 应用逻辑...
}
```

## 错误常量

| 常量             | 说明             |
| ---------------- | ---------------- |
| `ErrNotFound`    | 缓存键不存在     |
| `ErrLockNotHeld` | 锁未持有或已过期 |
| `ErrNilConfig`   | 配置为空         |
| `ErrEmptyAddr`   | 地址为空         |
| `ErrUnsupported` | 不支持的缓存类型 |
| `ErrNilLogger`   | logger 为空      |

## 类型常量

| 常量         | 值       | 说明       |
| ------------ | -------- | ---------- |
| `TypeRedis`  | `redis`  | Redis 缓存 |
| `TypeMemory` | `memory` | 内存缓存   |

## 测试

```bash
# 运行测试（内存缓存）
go test ./cache/... -v

# 运行测试（需要 Redis）
REDIS_ADDR=localhost:6379 go test ./cache/... -v
```

## License

MIT License
