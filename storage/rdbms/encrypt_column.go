package rdbms

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql/driver"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// EncryptColumn 数据库加密列类型，AES-GCM 加密 + base64 编码存储.
// 从数据库读取后需注入 Key 才能解密通常通过 GORM AfterFind 钩子.
//
//	type User struct {
//	    SSN EncryptColumn[string]
//	}
//
//	func (u *User) AfterFind(tx *gorm.DB) error {
//	    u.SSN.SetKey(getEncryptionKey())
//	    return nil
//	}
type EncryptColumn[T any] struct {
	Val        T
	Valid      bool
	key        string // AES 密钥16/24/32 字节，不导出以防止意外序列化
	Ciphertext string
}

// SetKey 设置 AES 密钥通常在 GORM AfterFind 钩子中调用.
func (ec *EncryptColumn[T]) SetKey(key string) {
	ec.key = key
}

// Key 返回当前 AES 密钥.
func (ec *EncryptColumn[T]) GetKey() string {
	return ec.key
}

// NewEncryptColumn 创建有效的加密列值.
// key 必须为 16/24/32 字节分别对应 AES-128/192/256.
func NewEncryptColumn[T any](val T, key string) (EncryptColumn[T], error) {
	if err := validateAESKey(key); err != nil {
		return EncryptColumn[T]{}, err
	}
	return EncryptColumn[T]{Val: val, Valid: true, key: key}, nil
}

// NullEncryptColumn 创建空值的加密列.
// key 为空时允许用于延迟注入密钥场景，非空时必须为 16/24/32 字节.
func NullEncryptColumn[T any](key string) (EncryptColumn[T], error) {
	if key != "" {
		if err := validateAESKey(key); err != nil {
			return EncryptColumn[T]{}, err
		}
	}
	return EncryptColumn[T]{key: key}, nil
}

func (ec EncryptColumn[T]) Value() (driver.Value, error) {
	if !ec.Valid {
		return nil, nil
	}

	plaintext, err := json.Marshal(ec.Val)
	if err != nil {
		return nil, fmt.Errorf("database: 加密列序列化失败: %w", err)
	}

	encrypted, err := aesGCMEncrypt(plaintext, []byte(ec.key))
	if err != nil {
		return nil, fmt.Errorf("database: 加密失败: %w", err)
	}

	return base64.StdEncoding.EncodeToString(encrypted), nil
}

func (ec *EncryptColumn[T]) Scan(src any) error {
	if src == nil {
		ec.Valid = false
		return nil
	}

	var encoded string
	switch v := src.(type) {
	case []byte:
		encoded = string(v)
	case string:
		encoded = v
	default:
		return fmt.Errorf("database: 加密列不支持类型 %T", src)
	}

	if ec.key == "" {
		ec.Ciphertext = encoded
		ec.Valid = false
		return nil
	}

	encrypted, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("database: base64 解码失败: %w", err)
	}

	plaintext, err := aesGCMDecrypt(encrypted, []byte(ec.key))
	if err != nil {
		return fmt.Errorf("database: 解密失败: %w", err)
	}

	if err := json.Unmarshal(plaintext, &ec.Val); err != nil {
		return fmt.Errorf("database: 加密列反序列化失败: %w", err)
	}
	ec.Valid = true
	return nil
}

// Decrypt 使用当前 Key 解密已保存的密文.
// 适用于 Scan 时 Key 为空、后续注入 Key 后调用的场景.
func (ec *EncryptColumn[T]) Decrypt() error {
	if ec.Valid {
		return nil
	}
	if ec.key == "" {
		return fmt.Errorf("database: 加密列 Key 为空，无法解密")
	}
	if ec.Ciphertext == "" {
		return fmt.Errorf("database: 加密列无密文可解密")
	}

	encrypted, err := base64.StdEncoding.DecodeString(ec.Ciphertext)
	if err != nil {
		return fmt.Errorf("database: base64 解码失败: %w", err)
	}

	plaintext, err := aesGCMDecrypt(encrypted, []byte(ec.key))
	if err != nil {
		return fmt.Errorf("database: 解密失败: %w", err)
	}

	if err := json.Unmarshal(plaintext, &ec.Val); err != nil {
		return fmt.Errorf("database: 加密列反序列化失败: %w", err)
	}
	ec.Valid = true
	ec.Ciphertext = ""
	return nil
}

func (EncryptColumn[T]) GormDBDataType(_ *gorm.DB, _ *schema.Field) string {
	return "TEXT"
}

// validateAESKey 校验 AES 密钥长度.
func validateAESKey(key string) error {
	if n := len(key); n != 16 && n != 24 && n != 32 {
		return fmt.Errorf("database: 无效的 AES 密钥长度 %d，必须为 16/24/32 字节", n)
	}
	return nil
}

func aesGCMEncrypt(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func aesGCMDecrypt(data, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("密文过短")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
