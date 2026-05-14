# lock

`github.com/Tsukikage7/servex/v2/storage/lock` -- 分布式锁。

## 概述

lock 包提供分布式锁实现，用于在多个进程或服务之间协调对共享资源的访问。当前提供基于 Redis 的实现，使用 SETNX + EXPIRE 保证原子性和过期机制。

## 功能特性

- 非阻塞尝试获取锁（TryLock）
- 阻塞等待获取锁（Lock），支持 context 取消与重试策略
- 安全释放：只有锁持有者才能释放锁
- 锁续期：长时间操作可在锁到期前延长有效期
- 辅助函数 WithLock / TryWithLock 简化加锁流程
- 每个锁实例自动生成唯一 ownerID，防止误释放

## API

### Locker 接口

| 方法 | 说明 |
|------|------|
| `TryLock(ctx, key, ttl) (bool, error)` | 尝试获取锁，失败立即返回 false |
| `Lock(ctx, key, ttl) error` | 阻塞获取锁，直到成功或 context 取消 |
| `Unlock(ctx, key) error` | 释放锁，仅持有者可释放 |
| `Extend(ctx, key, ttl) error` | 延长锁的过期时间 |

### Redis 实现

| 函数/方法 | 说明 |
|-----------|------|
| `NewRedis(cache, opts...) *Redis` | 创建 Redis 分布式锁 |
| `OwnerID() string` | 返回当前锁持有者 ID |
| `IsHeld(key) bool` | 检查是否持有指定锁 |

### 配置选项 (RedisOption)

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `WithKeyPrefix(prefix)` | `"lock:"` | 锁键前缀 |
| `WithOwnerID(id)` | 自动生成 UUID | 持有者 ID |
| `WithRetryWait(duration)` | `100ms` | 重试等待间隔 |
| `WithMaxRetries(n)` | `0`（无限重试） | 最大重试次数 |

### 辅助函数

| 函数 | 说明 |
|------|------|
| `WithLock(ctx, locker, key, ttl, fn) error` | 阻塞获取锁后执行操作 |
| `TryWithLock(ctx, locker, key, ttl, fn) error` | 非阻塞尝试获取锁后执行操作 |

### 预定义错误

| 错误 | 说明 |
|------|------|
| `ErrLockNotAcquired` | 无法获取锁 |
| `ErrLockNotHeld` | 锁未被当前实例持有 |
| `ErrLockExpired` | 锁已过期 |
