// Package randx 提供随机数和随机字符串等工具函数.
package randx

import (
	"crypto/rand"
	"encoding/binary"
	mrand "math/rand/v2"
)

const (
	asciiPrintable = "!\"#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`abcdefghijklmnopqrstuvwxyz{|}~"
	alphanumeric   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	alpha          = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digits         = "0123456789"
)

// Rand 随机数生成器.
type Rand struct {
	r *mrand.Rand
}

// New 创建使用 math/rand/v2 的高性能随机数生成器.
func New() *Rand {
	return &Rand{r: mrand.New(mrand.NewPCG(newSeed(), newSeed()))}
}

// NewSecure 创建使用 crypto/rand 作为熵源的安全随机数生成器.
// 每次生成随机数时都从 crypto/rand 读取，性能低于 New() 但适合安全场景.
func NewSecure() *Rand {
	src := newCryptoSource()
	return &Rand{r: mrand.New(src)}
}

// newSeed 从 crypto/rand 读取随机种子.
func newSeed() uint64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("randx: crypto/rand unavailable: " + err.Error())
	}
	return binary.LittleEndian.Uint64(b[:])
}

// cryptoSource 基于 crypto/rand 的 math/rand/v2 Source 实现.
type cryptoSource struct{}

func newCryptoSource() *cryptoSource { return &cryptoSource{} }

func (s *cryptoSource) Uint64() uint64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("randx: crypto/rand unavailable: " + err.Error())
	}
	return binary.LittleEndian.Uint64(b[:])
}

// RandInt 返回 [min, max) 范围内的随机整数.
// 若 min >= max，返回 min.
func (r *Rand) RandInt(min, max int) int {
	if min >= max {
		return min
	}
	return min + r.r.IntN(max-min)
}

// RandInt64 返回 [min, max) 范围内的随机 int64.
// 若 min >= max，返回 min.
func (r *Rand) RandInt64(min, max int64) int64 {
	if min >= max {
		return min
	}
	return min + r.r.Int64N(max-min)
}

// RandString 返回长度为 n 的可打印 ASCII 随机字符串.
func (r *Rand) RandString(n int) string {
	return r.randFrom(n, asciiPrintable)
}

// RandAlphanumeric 返回长度为 n 的 [a-zA-Z0-9] 随机字符串.
func (r *Rand) RandAlphanumeric(n int) string {
	return r.randFrom(n, alphanumeric)
}

// RandAlpha 返回长度为 n 的 [a-zA-Z] 随机字符串.
func (r *Rand) RandAlpha(n int) string {
	return r.randFrom(n, alpha)
}

// RandDigits 返回长度为 n 的 [0-9] 随机字符串.
func (r *Rand) RandDigits(n int) string {
	return r.randFrom(n, digits)
}

// randFrom 从给定字符集中随机选取 n 个字符组成字符串.
func (r *Rand) randFrom(n int, charset string) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = charset[r.r.IntN(len(charset))]
	}
	return string(b)
}

// RandElement 从切片中随机返回一个元素；切片为空时返回零值和 false.
func RandElement[T any](r *Rand, slice []T) (T, bool) {
	if len(slice) == 0 {
		var zero T
		return zero, false
	}
	return slice[r.r.IntN(len(slice))], true
}

// Sample 从切片中无放回随机采样 n 个元素n > len 时返回全部打乱副本.
func Sample[T any](r *Rand, slice []T, n int) []T {
	if len(slice) == 0 {
		return nil
	}
	cp := make([]T, len(slice))
	copy(cp, slice)
	Shuffle(r, cp)
	if n >= len(cp) {
		return cp
	}
	return cp[:n]
}

// Shuffle 原地使用 Fisher-Yates 算法打乱切片.
func Shuffle[T any](r *Rand, slice []T) {
	for i := len(slice) - 1; i > 0; i-- {
		j := r.r.IntN(i + 1)
		slice[i], slice[j] = slice[j], slice[i]
	}
}
