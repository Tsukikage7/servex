// Package crypto 提供随机数生成与密码哈希工具.
package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"golang.org/x/crypto/bcrypt"
)

// GenerateID 生成 32 位十六进制随机 ID.
func GenerateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// GenerateVerificationCode 生成 6 位数字验证码.
func GenerateVerificationCode() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		// crypto/rand 在正常系统上不会失败，但为保持接口不变做 fallback
		return "000000"
	}
	return fmt.Sprintf("%06d", n.Int64())
}

// GenerateRandomInt32 生成 [min, max] 范围内的随机 int32.
func GenerateRandomInt32(min, max int32) (int32, error) {
	if min >= max {
		return 0, errors.New("crypto: min must be less than max")
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max-min+1)))
	if err != nil {
		return 0, err
	}
	return min + int32(n.Int64()), nil
}

// GenerateRandomInt64 生成 [min, max] 范围内的随机 int64.
func GenerateRandomInt64(min, max int64) (int64, error) {
	if min >= max {
		return 0, errors.New("crypto: min must be less than max")
	}
	n, err := rand.Int(rand.Reader, big.NewInt(max-min+1))
	if err != nil {
		return 0, err
	}
	return min + n.Int64(), nil
}

// GenerateBusinessID 生成 9 位随机数字 ID [100000000, 999999999].
func GenerateBusinessID() int32 {
	n, err := rand.Int(rand.Reader, big.NewInt(900000000))
	if err != nil {
		return 100000000
	}
	return 100000000 + int32(n.Int64())
}

// GenerateBusinessID64 生成 18 位随机数字 ID [100000000000000000, 999999999999999999].
func GenerateBusinessID64() int64 {
	n, err := rand.Int(rand.Reader, big.NewInt(900000000000000000))
	if err != nil {
		return 100000000000000000
	}
	return 100000000000000000 + n.Int64()
}

// HashPassword 使用 bcrypt 对密码进行哈希.
func HashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

// VerifyPassword 验证密码是否匹配哈希值.
func VerifyPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}
