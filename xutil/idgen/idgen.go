// Package idgen 提供分布式 ID 生成器.
// 支持多种 ID 生成算法：
//   - Snowflake: 41-bit 时间戳 + 5-bit 数据中心 + 5-bit 工作节点 + 12-bit 序列号
//   - ULID: 26 字符 Crockford Base32，同毫秒内单调递增
//   - NanoID: 可配置字母表和长度的随机 ID
//   - UUID: 封装 google/uuid
//
// 示例：
//
//	// Snowflake
//	gen, _ := idgen.NewSnowflake(&idgen.SnowflakeConfig{WorkerID: 1})
//	id, _ := gen.NextID() // "6849812345678901"
//	// 便捷函数
//	id := idgen.ULID()   // "01ARZ3NDEKTSV4RRFFQ69G5FAV"
//	id = idgen.NanoID()   // "V1StGXR8_Z5jdHi6B-myT"
//	id = idgen.UUID()     // "550e8400-e29b-41d4-a716-446655440000"
package idgen

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrInvalidWorkerID 无效的 Worker ID.
	ErrInvalidWorkerID = errors.New("idgen: worker ID must be 0-1023")
	// ErrInvalidDatacenterID 无效的 Datacenter ID.
	ErrInvalidDatacenterID = errors.New("idgen: datacenter ID must be 0-31")
)

// Generator ID 生成器接口.
type Generator interface {
	NextID() (string, error)
}

// ============================================================================
// Snowflake
// ============================================================================

const (
	snowflakeTimestampBits  = 41
	snowflakeDatacenterBits = 5
	snowflakeWorkerBits     = 5
	snowflakeSequenceBits   = 12

	snowflakeMaxDatacenterID = -1 ^ (-1 << snowflakeDatacenterBits)                         // 31
	snowflakeMaxWorkerID     = -1 ^ (-1 << (snowflakeDatacenterBits + snowflakeWorkerBits)) // 1023 (combined)
	snowflakeMaxSequence     = -1 ^ (-1 << snowflakeSequenceBits)                           // 4095

	snowflakeWorkerShift     = snowflakeSequenceBits
	snowflakeDatacenterShift = snowflakeSequenceBits + snowflakeWorkerBits
	snowflakeTimestampShift  = snowflakeSequenceBits + snowflakeWorkerBits + snowflakeDatacenterBits
)

// SnowflakeConfig Snowflake 配置.
type SnowflakeConfig struct {
	WorkerID     int64     // 0-1023
	DatacenterID int64     // 0-31
	Epoch        time.Time // 自定义纪元，默认 2020-01-01
}

type snowflakeGen struct {
	mu           sync.Mutex
	epoch        int64
	workerID     int64
	datacenterID int64
	sequence     int64
	lastTime     int64
}

// NewSnowflake 创建 Snowflake ID 生成器.
func NewSnowflake(cfg *SnowflakeConfig) (Generator, error) {
	if cfg == nil {
		cfg = &SnowflakeConfig{}
	}
	if cfg.WorkerID < 0 || cfg.WorkerID > 1023 {
		return nil, ErrInvalidWorkerID
	}
	if cfg.DatacenterID < 0 || cfg.DatacenterID > 31 {
		return nil, ErrInvalidDatacenterID
	}

	epoch := cfg.Epoch
	if epoch.IsZero() {
		epoch = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	return &snowflakeGen{
		epoch:        epoch.UnixMilli(),
		workerID:     cfg.WorkerID,
		datacenterID: cfg.DatacenterID,
	}, nil
}

func (s *snowflakeGen) NextID() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli() - s.epoch

	if now == s.lastTime {
		s.sequence = (s.sequence + 1) & snowflakeMaxSequence
		if s.sequence == 0 {
			// 序列号用尽，等待下一毫秒
			for now <= s.lastTime {
				now = time.Now().UnixMilli() - s.epoch
			}
		}
	} else {
		s.sequence = 0
	}

	s.lastTime = now

	id := (now << snowflakeTimestampShift) |
		(s.datacenterID << snowflakeDatacenterShift) |
		(s.workerID << snowflakeWorkerShift) |
		s.sequence

	return strconv.FormatInt(id, 10), nil
}

// ============================================================================
// ULID
// ============================================================================

// Crockford Base32 字符表.
const crockfordBase32 = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

type ulidGen struct {
	mu      sync.Mutex
	lastMs  int64
	lastRnd [10]byte // 80-bit 随机部分
}

// NewULID 创建 ULID 生成器.
// 使用 crypto/rand，同毫秒内单调递增.
func NewULID() Generator {
	return &ulidGen{}
}

func (u *ulidGen) NextID() (string, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	ms := time.Now().UnixMilli()

	if ms == u.lastMs {
		// 同毫秒，递增随机部分
		for i := 9; i >= 0; i-- {
			u.lastRnd[i]++
			if u.lastRnd[i] != 0 {
				break
			}
		}
	} else {
		u.lastMs = ms
		// 生成新的随机部分
		if _, err := rand.Read(u.lastRnd[:]); err != nil {
			return "", fmt.Errorf("idgen: failed to generate random bytes: %w", err)
		}
	}

	// 编码: 10 字符时间戳 + 16 字符随机部分 = 26 字符
	var buf [26]byte

	// 编码时间戳（48-bit → 10 个 Base32 字符）
	ts := uint64(ms)
	for i := 9; i >= 0; i-- {
		buf[i] = crockfordBase32[ts&0x1F]
		ts >>= 5
	}

	// 编码随机部分（80-bit → 16 个 Base32 字符）
	// 将 10 字节转为位流处理
	rnd := u.lastRnd
	// 使用简单的逐 5-bit 提取
	// 80 bits = 16 * 5 bits
	bits := uint64(0)
	bitsLen := 0
	rndIdx := 0
	for i := 0; i < 16; i++ {
		for bitsLen < 5 {
			if rndIdx < 10 {
				bits = (bits << 8) | uint64(rnd[rndIdx])
				bitsLen += 8
				rndIdx++
			}
		}
		bitsLen -= 5
		buf[10+i] = crockfordBase32[(bits>>uint(bitsLen))&0x1F]
	}

	return string(buf[:]), nil
}

// ============================================================================
// NanoID
// ============================================================================

const defaultNanoAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-"
const defaultNanoSize = 21

// NanoOption NanoID 配置选项.
type NanoOption func(*nanoOptions)

type nanoOptions struct {
	alphabet string
	size     int
}

// WithAlphabet 设置 NanoID 字母表.
func WithAlphabet(alphabet string) NanoOption {
	return func(o *nanoOptions) {
		o.alphabet = alphabet
	}
}

// WithSize 设置 NanoID 长度.
func WithSize(size int) NanoOption {
	return func(o *nanoOptions) {
		o.size = size
	}
}

type nanoGen struct {
	alphabet string
	size     int
}

// NewNanoID 创建 NanoID 生成器.
func NewNanoID(opts ...NanoOption) Generator {
	o := &nanoOptions{
		alphabet: defaultNanoAlphabet,
		size:     defaultNanoSize,
	}
	for _, opt := range opts {
		opt(o)
	}
	return &nanoGen{
		alphabet: o.alphabet,
		size:     o.size,
	}
}

func (n *nanoGen) NextID() (string, error) {
	alphabetLen := big.NewInt(int64(len(n.alphabet)))
	buf := make([]byte, n.size)
	for i := 0; i < n.size; i++ {
		idx, err := rand.Int(rand.Reader, alphabetLen)
		if err != nil {
			return "", fmt.Errorf("idgen: failed to generate random index: %w", err)
		}
		buf[i] = n.alphabet[idx.Int64()]
	}
	return string(buf), nil
}

// ============================================================================
// 便捷函数
// ============================================================================

var (
	defaultSnowflake Generator
	defaultULID      Generator
	defaultNanoID    Generator
	initOnce         sync.Once
)

func initDefaults() {
	initOnce.Do(func() {
		var err error
		defaultSnowflake, err = NewSnowflake(&SnowflakeConfig{})
		if err != nil {
			panic(fmt.Sprintf("idgen: failed to init default snowflake: %v", err))
		}
		defaultULID = NewULID()
		defaultNanoID = NewNanoID()
	})
}

// Snowflake 使用默认配置生成 Snowflake ID.
// 出错时 panic.
func Snowflake() string {
	initDefaults()
	id, err := defaultSnowflake.NextID()
	if err != nil {
		panic(fmt.Sprintf("idgen: snowflake error: %v", err))
	}
	return id
}

// ULID 生成 ULID.
func ULID() string {
	initDefaults()
	id, err := defaultULID.NextID()
	if err != nil {
		panic(fmt.Sprintf("idgen: ulid error: %v", err))
	}
	return id
}

// NanoID 生成 NanoID.
func NanoID() string {
	initDefaults()
	id, err := defaultNanoID.NextID()
	if err != nil {
		panic(fmt.Sprintf("idgen: nanoid error: %v", err))
	}
	return id
}

// UUID 生成 UUID v4.
func UUID() string {
	return uuid.New().String()
}

// SnowflakeE 使用默认配置生成 Snowflake ID（返回 error 而非 panic）.
func SnowflakeE() (string, error) {
	initDefaults()
	return defaultSnowflake.NextID()
}

// ULIDE 生成 ULID（返回 error 而非 panic）.
func ULIDE() (string, error) {
	initDefaults()
	return defaultULID.NextID()
}

// NanoIDE 生成 NanoID（返回 error 而非 panic）.
func NanoIDE() (string, error) {
	initDefaults()
	return defaultNanoID.NextID()
}
