package syncx

import (
	"math/bits"
	"sync"
)

// SegmentKeysLock 分段键锁.
// 通过对 key 进行哈希分段，减小锁粒度提升并发性能.
type SegmentKeysLock struct {
	locks []sync.RWMutex
	size  uint32
}

// NewSegmentKeysLock 创建分段键锁.
// size 会被向上取整到 2 的幂次，以保证哈希分布均匀.
func NewSegmentKeysLock(size uint32) *SegmentKeysLock {
	if size == 0 {
		size = 16
	}
	// 向上取整到 2 的幂次
	size = nextPowerOf2(size)
	return &SegmentKeysLock{
		locks: make([]sync.RWMutex, size),
		size:  size,
	}
}

// nextPowerOf2 返回 >= n 的最小 2 的幂次.
func nextPowerOf2(n uint32) uint32 {
	if n == 0 {
		return 1
	}
	// 如果已经是 2 的幂次则直接返回
	if bits.OnesCount32(n) == 1 {
		return n
	}
	return 1 << bits.Len32(n)
}

// Lock 对指定 key 加写锁.
func (s *SegmentKeysLock) Lock(key string) { s.locks[s.hash(key)].Lock() }

// TryLock 尝试对指定 key 加写锁，成功返回 true.
func (s *SegmentKeysLock) TryLock(key string) bool { return s.locks[s.hash(key)].TryLock() }

// Unlock 释放指定 key 的写锁.
func (s *SegmentKeysLock) Unlock(key string) { s.locks[s.hash(key)].Unlock() }

// RLock 对指定 key 加读锁.
func (s *SegmentKeysLock) RLock(key string) { s.locks[s.hash(key)].RLock() }

// TryRLock 尝试对指定 key 加读锁，成功返回 true.
func (s *SegmentKeysLock) TryRLock(key string) bool { return s.locks[s.hash(key)].TryRLock() }

// RUnlock 释放指定 key 的读锁.
func (s *SegmentKeysLock) RUnlock(key string) { s.locks[s.hash(key)].RUnlock() }

// hash FNV-1a 零分配实现.
func (s *SegmentKeysLock) hash(key string) uint32 {
	const (
		offset32 = uint32(2166136261)
		prime32  = uint32(16777619)
	)
	h := offset32
	for i := range len(key) {
		h ^= uint32(key[i])
		h *= prime32
	}
	return h & (s.size - 1) // size 保证为 2 的幂次，位与替代取模更高效且分布均匀
}
